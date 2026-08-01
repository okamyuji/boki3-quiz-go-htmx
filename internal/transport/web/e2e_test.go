// Package web の E2E 風統合テスト。
//
// 実際の DB (一時 sqlite) と全レイヤを配線して、HTTP 経由のユースケースを
// 検証する。Playwright ではなく net/http/httptest で十分検証可能なフロー。
package web_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/idgen"
	jwtauth "github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/jwt"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/password"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/ratelimit"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/clock"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/sqlitex"
	reposqlite "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/service"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/transport/api"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/transport/middleware"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/transport/web"
)

type e2eFixture struct {
	srv    *httptest.Server
	client *http.Client
	db     *sql.DB
}

func setupE2E(t *testing.T) *e2eFixture {
	t.Helper()
	dir := t.TempDir()
	db, err := sqlitex.Open("file:" + filepath.Join(dir, "e2e.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if err := reposqlite.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	seedMinimalQuestions(t, db)

	users := reposqlite.NewUserRepo(db)
	sessions := reposqlite.NewSessionRepo(db)
	jwts := reposqlite.NewJWTRevocationRepo(db)
	questions := reposqlite.NewQuestionRepo(db)
	sets := reposqlite.NewSetRepo(db)
	topics := reposqlite.NewTopicRepo(db)
	attempts := reposqlite.NewAttemptRepo(db)
	srss := reposqlite.NewSRSStateRepo(db)

	hasher := password.New(password.Params{N: 1024, R: 8, P: 1, KeyLen: 32, SaltLen: 16})
	g := idgen.New()
	clk := clock.System{}

	authSvc := service.NewAuthService(users, sessions, jwts, hasher, g, clk, service.DefaultAuthConfig())
	signer, _ := jwtauth.NewHS256([]byte("test-secret-32bytes-test-secret-32bytes"), clk)
	apiAuth := service.NewAPIAuthService(signer, jwts, g, clk, "boki3-quiz", "api")
	quizSvc := service.NewQuizService(questions, sets, attempts, srss, clk)
	statsSvc := service.NewStatsService(attempts, srss, clk)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tpl, err := web.LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}

	mux := http.NewServeMux()
	webH := web.NewHandler(web.Config{
		Templates: tpl, Auth: authSvc, Quiz: quizSvc, Stats: statsSvc,
		Sets: sets, Prefs: reposqlite.NewUserPrefsRepo(db), Questions: questions, Topics: topics, Logger: logger,
		LoginRateLimit:  ratelimit.NewFixedWindow(100, time.Minute, clk),
		GlobalRateLimit: ratelimit.NewSlidingWindow(1000, time.Minute, clk),
		Cookie:          web.CookieConfig{SessionName: "boki3_session", CSRFName: "boki3_csrf"},
		StartedAtSecret: []byte("test-started-at-secret-32bytes-pad"),
	})
	webH.Register(mux)
	apiH := api.NewHandler(api.Config{
		Auth: authSvc, API: apiAuth, Quiz: quizSvc, Stats: statsSvc,
		Questions: questions, Logger: logger, TokenTTL: time.Hour,
	})
	apiH.Register(mux)

	chain := middleware.Chain(mux,
		middleware.Recover(logger),
		middleware.RequestID(),
		middleware.SecurityHeaders(logger),
		middleware.BodyLimit(1<<20),
	)

	srv := httptest.NewServer(chain)
	t.Cleanup(srv.Close)

	jar, _ := newJar()
	client := &http.Client{
		Jar: jar,
		// follow redirects but not infinitely
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) > 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	return &e2eFixture{srv: srv, client: client, db: db}
}

// seedMinimalQuestions は最小限の 1 問を投入する (テスト固有のシード)。
func seedMinimalQuestions(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `INSERT INTO topics(code, name, ord) VALUES (?, ?, ?)`, "cash", "現金", 1)
	if err != nil {
		t.Fatalf("seed topic: %v", err)
	}
	answer, _ := json.Marshal(domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "現金", Amount: 1000}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 1000}},
	})
	res, err := db.ExecContext(ctx,
		`INSERT INTO questions(code, topic_id, question_type, difficulty, prompt, payload_json, answer_json, explanation, references_json, created_at)
		 VALUES (?, 1, 'journal', 1, ?, '{}', ?, ?, NULL, 0)`,
		"e2e-q1", "現金で 1000 円を売上げた。", string(answer), "現金売上の仕訳。")
	if err != nil {
		t.Fatalf("seed question: %v", err)
	}
	qid, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx,
		`INSERT INTO question_sets(code, name, description, target_size) VALUES (?, ?, '', ?)`,
		"core_300", "コア300", 300)
	if err != nil {
		t.Fatalf("seed set: %v", err)
	}
	setID, _ := res.LastInsertId()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO question_set_members(set_id, question_id, ord) VALUES (?, ?, 1)`, setID, qid); err != nil {
		t.Fatalf("seed member: %v", err)
	}
}

// 簡易 cookie jar (net/http/cookiejar の代替: 単一ドメインのみ)。
type simpleJar struct {
	cookies map[string]*http.Cookie
}

func newJar() (*simpleJar, error) {
	return &simpleJar{cookies: map[string]*http.Cookie{}}, nil
}

func (j *simpleJar) SetCookies(_ *url.URL, cookies []*http.Cookie) {
	for _, c := range cookies {
		if c.MaxAge < 0 {
			delete(j.cookies, c.Name)
			continue
		}
		j.cookies[c.Name] = c
	}
}

func (j *simpleJar) Cookies(_ *url.URL) []*http.Cookie {
	out := make([]*http.Cookie, 0, len(j.cookies))
	for _, c := range j.cookies {
		out = append(out, c)
	}
	return out
}

func TestE2ERegisterLoginQuizFlow(t *testing.T) {
	t.Parallel()
	fx := setupE2E(t)

	// 1) GET /register
	resp := mustGet(t, fx, "/register")
	csrf := extractCSRF(t, resp)
	_ = resp.Body.Close()

	// 2) POST /register
	body := url.Values{
		"csrf_token": {csrf},
		"username":   {"alice01"},
		"password":   {"P@ssw0rd!Strong"},
	}
	resp = mustPostForm(t, fx, "/register", body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("/register status = %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// 3) GET /quiz (認証必須なのでアクセス可能になっているはず)
	resp = mustGet(t, fx, "/quiz")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/quiz status = %d, want 200", resp.StatusCode)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(bodyBytes), "現金で 1000 円を売上げた") {
		t.Fatalf("/quiz body does not contain prompt; body=%s", string(bodyBytes)[:min(400, len(bodyBytes))])
	}
}

// registerE2EUser は登録フローを通してログイン済みセッションを作る。
func registerE2EUser(t *testing.T, fx *e2eFixture, username string) {
	t.Helper()
	resp := mustGet(t, fx, "/register")
	csrf := extractCSRF(t, resp)
	_ = resp.Body.Close()
	resp = mustPostForm(t, fx, "/register", url.Values{
		"csrf_token": {csrf},
		"username":   {username},
		"password":   {"P@ssw0rd!Strong"},
	})
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("/register status = %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
}

func TestE2EQuizSetModeRestoredFromSavedPrefs(t *testing.T) {
	t.Parallel()
	fx := setupE2E(t)
	// 2 つ目のセットを用意して切り替え先にする。
	ctx := context.Background()
	res, err := fx.db.ExecContext(ctx,
		`INSERT INTO question_sets(code, name, description, target_size) VALUES ('journal_150', '仕訳150', '', 150)`)
	if err != nil {
		t.Fatalf("seed second set: %v", err)
	}
	setID, _ := res.LastInsertId()
	if _, err := fx.db.ExecContext(ctx,
		`INSERT INTO question_set_members(set_id, question_id, ord) SELECT ?, id, 1 FROM questions LIMIT 1`, setID); err != nil {
		t.Fatalf("seed second set member: %v", err)
	}
	registerE2EUser(t, fx, "prefsuser1")

	// 切り替え (明示クエリ) → 保存される。
	resp := mustGet(t, fx, "/quiz?set=journal_150&mode=random")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/quiz with query status = %d, want 200", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// パラメータ無しの再訪問 (ログインし直し相当) で保存値が復帰する。
	resp = mustGet(t, fx, "/quiz")
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	body := string(b)
	if !strings.Contains(body, `value="journal_150" selected`) {
		t.Fatalf("saved set not restored; body=%s", body[:min(600, len(body))])
	}
	if !strings.Contains(body, `value="random" selected`) {
		t.Fatalf("saved mode not restored; body=%s", body[:min(600, len(body))])
	}

	// 無効なクエリ値は無視され、保存済みの選択が使われる (保存値も汚染されない)。
	resp = mustGet(t, fx, "/quiz?set=no_such_set&mode=bogus")
	b, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	body = string(b)
	if !strings.Contains(body, `value="journal_150" selected`) {
		t.Fatalf("invalid set query must fall back to stored; body=%s", body[:min(600, len(body))])
	}
	if !strings.Contains(body, `value="random" selected`) {
		t.Fatalf("invalid mode query must fall back to stored; body=%s", body[:min(600, len(body))])
	}
}

func TestE2EQuizDefaultsForNewUserWithoutPrefs(t *testing.T) {
	t.Parallel()
	fx := setupE2E(t)
	registerE2EUser(t, fx, "prefsuser2")

	resp := mustGet(t, fx, "/quiz")
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	body := string(b)
	if !strings.Contains(body, `value="core_300" selected`) {
		t.Fatalf("default set not selected; body=%s", body[:min(600, len(body))])
	}
	if !strings.Contains(body, `value="srs" selected`) {
		t.Fatalf("default mode not selected; body=%s", body[:min(600, len(body))])
	}
}

func TestE2EHomePageRenders(t *testing.T) {
	t.Parallel()
	fx := setupE2E(t)
	resp := mustGet(t, fx, "/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/ = %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(b), "簿記3級") {
		t.Fatalf("home page does not contain expected title")
	}
}

func TestE2EUnauthenticatedRedirects(t *testing.T) {
	t.Parallel()
	fx := setupE2E(t)
	resp, err := fx.client.Get(fx.srv.URL + "/quiz")
	if err != nil {
		t.Fatalf("GET /quiz: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// 未認証は /login へ redirect (CheckRedirect で follow される)
	if !strings.HasSuffix(resp.Request.URL.Path, "/login") {
		t.Fatalf("final URL = %s, want ending in /login", resp.Request.URL.Path)
	}
}

func TestE2ESecurityHeadersPresent(t *testing.T) {
	t.Parallel()
	fx := setupE2E(t)
	resp := mustGet(t, fx, "/")
	defer func() { _ = resp.Body.Close() }()
	for _, k := range []string{"Content-Security-Policy", "X-Frame-Options", "Strict-Transport-Security"} {
		if resp.Header.Get(k) == "" {
			t.Fatalf("missing header %q", k)
		}
	}
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("X-Frame-Options = %q", resp.Header.Get("X-Frame-Options"))
	}
}

func TestE2EAPILoginAndNext(t *testing.T) {
	t.Parallel()
	fx := setupE2E(t)
	// ユーザ登録は HTML 経路でしか提供していないため、まず Web 経由で登録する。
	resp := mustGet(t, fx, "/register")
	csrf := extractCSRF(t, resp)
	_ = resp.Body.Close()
	resp = mustPostForm(t, fx, "/register", url.Values{
		"csrf_token": {csrf},
		"username":   {"apiuser01"},
		"password":   {"P@ssw0rd!Strong"},
	})
	_ = resp.Body.Close()

	// JWT 取得
	body := strings.NewReader(`{"username":"apiuser01","password":"P@ssw0rd!Strong"}`)
	req, _ := http.NewRequest(http.MethodPost, fx.srv.URL+"/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	r, err := fx.client.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/auth/login: %v", err)
	}
	defer func() { _ = r.Body.Close() }()
	if r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("API login status = %d, body=%s", r.StatusCode, string(b))
	}
	var lr struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&lr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if lr.Token == "" {
		t.Fatalf("empty token")
	}

	// /api/v1/quiz/next
	req2, _ := http.NewRequest(http.MethodGet, fx.srv.URL+"/api/v1/quiz/next?set=core_300", http.NoBody)
	req2.Header.Set("Authorization", "Bearer "+lr.Token)
	r2, err := fx.client.Do(req2)
	if err != nil {
		t.Fatalf("GET /api/v1/quiz/next: %v", err)
	}
	defer func() { _ = r2.Body.Close() }()
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("next status = %d", r2.StatusCode)
	}
}

// --- helpers ---

func mustGet(t *testing.T, fx *e2eFixture, path string) *http.Response {
	t.Helper()
	r, err := fx.client.Get(fx.srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return r
}

func mustPostForm(t *testing.T, fx *e2eFixture, path string, values url.Values) *http.Response {
	t.Helper()
	r, err := fx.client.PostForm(fx.srv.URL+path, values)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return r
}

// extractCSRF は HTML から最初の csrf_token hidden input を取り出す簡易抽出。
func extractCSRF(t *testing.T, r *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	needle := []byte(`name="csrf_token" value="`)
	idx := bytes.Index(b, needle)
	if idx < 0 {
		t.Fatalf("csrf hidden input not found")
	}
	rest := b[idx+len(needle):]
	end := bytes.IndexByte(rest, '"')
	if end < 0 {
		t.Fatalf("csrf value not terminated")
	}
	return string(rest[:end])
}
