// Package main is the entrypoint of the boki3-quiz HTTP server.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/idgen"
	jwtauth "github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/jwt"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/password"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/ratelimit"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/clock"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/sqlitex"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/version"
	reposqlite "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/service"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/transport/api"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/transport/middleware"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/transport/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server terminated", "err", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	addr := envOr("BOKI3_LISTEN", ":8080")
	dbPath := envOr("BOKI3_DB_PATH", "boki3-quiz.db")
	jwtSecret, err := loadJWTSecret(os.Getenv("BOKI3_JWT_SECRET"))
	if err != nil {
		return err
	}

	db, err := sqlitex.Open("file:" + dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := reposqlite.Migrate(context.Background(), db); err != nil {
		return err
	}

	// Repositories
	users := reposqlite.NewUserRepo(db)
	sessions := reposqlite.NewSessionRepo(db)
	jwts := reposqlite.NewJWTRevocationRepo(db)
	questions := reposqlite.NewQuestionRepo(db)
	sets := reposqlite.NewSetRepo(db)
	attempts := reposqlite.NewAttemptRepo(db)
	srss := reposqlite.NewSRSStateRepo(db)

	// Auth pieces
	hasher := password.Default()
	idg := idgen.New()
	signer, err := jwtauth.NewHS256(jwtSecret)
	if err != nil {
		return err
	}
	clk := clock.System{}

	// Services
	authSvc := service.NewAuthService(users, sessions, jwts, hasher, idg, clk, service.DefaultAuthConfig())
	apiAuth := service.NewAPIAuthService(signer, jwts, idg, clk, "boki3-quiz", "api")
	quizSvc := service.NewQuizService(questions, sets, attempts, srss, clk)
	statsSvc := service.NewStatsService(attempts, srss, clk)

	// Rate limiters
	globalRL := ratelimit.NewSlidingWindow(120, time.Minute, clk)
	loginRL := ratelimit.NewFixedWindow(5, 10*time.Minute, clk)
	userAPIRL := ratelimit.NewTokenBucket(60, 1, clk)

	// Templates
	tpl, err := web.LoadTemplates()
	if err != nil {
		return err
	}

	// Cookie config
	cookieCfg := web.CookieConfig{
		SessionName: "boki3_session",
		CSRFName:    "boki3_csrf",
		Secure:      envBool("BOKI3_COOKIE_SECURE", false),
		Domain:      os.Getenv("BOKI3_COOKIE_DOMAIN"),
	}

	// Handlers
	mux := http.NewServeMux()
	webH := web.NewHandler(web.Config{
		Templates: tpl,
		Auth:      authSvc, Quiz: quizSvc, Stats: statsSvc,
		Sets: sets, Questions: questions, Logger: logger,
		LoginRateLimit: loginRL, GlobalRateLimit: globalRL,
		Cookie: cookieCfg,
	})
	webH.Register(mux)
	apiH := api.NewHandler(api.Config{
		Auth: authSvc, API: apiAuth, Quiz: quizSvc, Stats: statsSvc,
		Questions: questions, Logger: logger,
		TokenTTL: time.Hour, UserRateLimit: userAPIRL,
	})
	apiH.Register(mux)

	// Static assets
	staticFS, err := web.StaticFS()
	if err != nil {
		return err
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(noListFS{fs: staticFS}))))

	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", healthz)
	mux.HandleFunc("GET /version", versionHandler)

	// CORS
	allowedOrigins := splitEnv("BOKI3_CORS_ORIGINS")
	chain := middleware.Chain(mux,
		middleware.Recover(logger),
		middleware.RequestID(),
		middleware.AccessLog(logger),
		middleware.SecurityHeaders(),
		middleware.BodyLimit(1<<20),
		middleware.RateLimitByIP(globalRL),
		middleware.CORS(middleware.CORSConfig{
			AllowedOrigins: allowedOrigins,
			AllowCreds:     true,
			MaxAge:         600,
		}),
	)

	srv := &http.Server{
		Addr:              addr,
		Handler:           chain,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", "addr", addr, "version", version.String())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// noListFS は fs.FS を embed.FS のディレクトリインデックスを返さないようにラップする。
type noListFS struct{ fs fs.FS }

func (n noListFS) Open(name string) (fs.File, error) {
	if strings.HasSuffix(name, "/") {
		return nil, fs.ErrNotExist
	}
	return n.fs.Open(name)
}

// newRouter は最小限の health/version ハンドラ用 mux を返す (テスト用)。
func newRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthz)
	mux.HandleFunc("/readyz", healthz)
	mux.HandleFunc("/version", versionHandler)
	return mux
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok\n")); err != nil {
		slog.Warn("write healthz body", "err", err)
	}
}

func versionHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(version.String() + "\n")); err != nil {
		slog.Warn("write version body", "err", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := strings.ToLower(os.Getenv(key))
	switch v {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	}
	return def
}

func splitEnv(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadJWTSecret は BOKI3_JWT_SECRET (hex 64+ 文字) を読む。未設定なら起動時に 32 バイトを乱数生成する。
// 生成した場合は警告ログを出す (本番では設定必須)。
func loadJWTSecret(envVal string) ([]byte, error) {
	if envVal != "" {
		b, err := hex.DecodeString(envVal)
		if err != nil {
			return nil, errors.New("BOKI3_JWT_SECRET must be hex-encoded")
		}
		if len(b) < 32 {
			return nil, errors.New("BOKI3_JWT_SECRET must be >= 32 bytes")
		}
		return b, nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	slog.Warn("BOKI3_JWT_SECRET not set; using ephemeral random secret (tokens invalid after restart)")
	return b, nil
}
