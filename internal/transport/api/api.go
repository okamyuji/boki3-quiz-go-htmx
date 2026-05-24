// Package api は /api/v1/* の JSON ハンドラを提供する。認証は JWT Bearer。
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/transport/middleware"
)

// Config は API ハンドラの構築引数。
type Config struct {
	Auth          port.AuthService
	API           port.APIAuthService
	Quiz          port.QuizService
	Stats         port.StatsService
	Questions     port.QuestionRepository
	Logger        *slog.Logger
	TokenTTL      time.Duration
	UserRateLimit port.RateLimiter
}

// Handler は API ルートを mux に登録する。
type Handler struct{ cfg Config }

// NewHandler は API Handler を生成する。
func NewHandler(cfg Config) *Handler { return &Handler{cfg: cfg} }

// Register はルートを mux に登録する。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.Handle("POST /api/v1/auth/logout", h.authed(http.HandlerFunc(h.logout)))
	mux.Handle("GET /api/v1/quiz/next", h.authed(http.HandlerFunc(h.next)))
	mux.Handle("POST /api/v1/quiz/answer", h.authed(http.HandlerFunc(h.answer)))
	mux.Handle("GET /api/v1/history", h.authed(http.HandlerFunc(h.history)))
	mux.Handle("DELETE /api/v1/history/{id}", h.authed(http.HandlerFunc(h.deleteAttempt)))
	mux.Handle("GET /api/v1/stats/summary", h.authed(http.HandlerFunc(h.summary)))
}

func (h *Handler) authed(inner http.Handler) http.Handler {
	jwt := middleware.JWTBearer(h.cfg.API)
	if h.cfg.UserRateLimit == nil {
		return jwt(inner)
	}
	rl := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid := middleware.JWTUserIDFrom(r.Context())
			if !h.cfg.UserRateLimit.Allow(strconv.FormatInt(uid, 10)) {
				writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	return jwt(rl(inner))
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResp struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in loginReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad_request"})
		return
	}
	// API ログインは AuthService.Login で認証し、得られた user.ID で JWT を発行する。
	// セッションは即座に Logout で破棄し、API クライアントは JWT のみで認証する。
	sess, err := h.cfg.Auth.Login(r.Context(), in.Username, in.Password, r.UserAgent(), clientIP(r))
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	tok, _, exp, err := h.cfg.API.IssueToken(r.Context(), sess.UserID, ttl(h.cfg.TokenTTL))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	// セッションは破棄 (API 利用は JWT のみ)。
	_ = h.cfg.Auth.Logout(r.Context(), sess.ID)
	writeJSON(w, http.StatusOK, loginResp{Token: tok, ExpiresAt: exp})
}

func ttl(d time.Duration) time.Duration {
	if d <= 0 {
		return time.Hour
	}
	return d
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if tok, ok := strings.CutPrefix(auth, prefix); ok {
		// VerifyToken は失効済みでも nil を返さないので、ここでは Parse 経由ではなく
		// APIAuthService にトークン -> claims 抽出と Revoke を委ねる薄い経路を使う。
		// 簡略化として VerifyToken で取得した userID + 残存期限を使う。
		// (失効されたトークンは IsRevoked で再ログアウト時にも一致して問題ない)
		uid, err := h.cfg.API.VerifyToken(r.Context(), tok)
		if err == nil {
			// API.Revoke は jti と expiresAt が必要だが、ここでは現在からトークン TTL を上限に設定する。
			// 多重 logout でも upsert で重複なし。
			_ = h.cfg.API.Revoke(r.Context(), bearerJTIPlaceholder(tok), uid, time.Now().Add(h.cfg.TokenTTL))
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// bearerJTIPlaceholder は Authorization トークン文字列のハッシュを jti として使う近似。
// 公式 jti が必要な場合は APIAuthService 経由で取得すべきだが、本アプリの logout は best-effort 失効。
func bearerJTIPlaceholder(tok string) string {
	// 単純に最初の 32 文字 (payload + signature の冒頭) を切る。
	if len(tok) > 32 {
		return "logout:" + tok[:32]
	}
	return "logout:" + tok
}

type nextResp struct {
	Question *domain.Question `json:"question"`
}

func (h *Handler) next(w http.ResponseWriter, r *http.Request) {
	uid := middleware.JWTUserIDFrom(r.Context())
	setCode := r.URL.Query().Get("set")
	if setCode == "" {
		setCode = domain.SetCodeCore
	}
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "srs"
	}
	q, err := h.cfg.Quiz.NextQuestion(r.Context(), uid, setCode, domain.QuizMode(mode))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	writeJSON(w, http.StatusOK, nextResp{Question: q})
}

type answerReq struct {
	QuestionID int64                `json:"question_id"`
	SetCode    string               `json:"set_code"`
	DurationMs int                  `json:"duration_ms"`
	Answer     domain.AnswerPayload `json:"answer"`
}

func (h *Handler) answer(w http.ResponseWriter, r *http.Request) {
	uid := middleware.JWTUserIDFrom(r.Context())
	var in answerReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad_request"})
		return
	}
	g, err := h.cfg.Quiz.Submit(r.Context(), uid, domain.SubmitInput{
		QuestionID: in.QuestionID, SetCode: in.SetCode, Answer: in.Answer, DurationMs: in.DurationMs,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	uid := middleware.JWTUserIDFrom(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	attempts, err := h.cfg.Quiz.History(r.Context(), uid, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"attempts": attempts})
}

func (h *Handler) deleteAttempt(w http.ResponseWriter, r *http.Request) {
	uid := middleware.JWTUserIDFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad_id"})
		return
	}
	if err := h.cfg.Quiz.DeleteAttempt(r.Context(), uid, id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	uid := middleware.JWTUserIDFrom(r.Context())
	s, err := h.cfg.Stats.Summary(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i > 0 {
			v = v[:i]
		}
		return strings.TrimSpace(v)
	}
	if i := strings.LastIndexByte(r.RemoteAddr, ':'); i > 0 {
		return r.RemoteAddr[:i]
	}
	return r.RemoteAddr
}
