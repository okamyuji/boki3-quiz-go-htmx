package web

import (
	"context"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/transport/middleware"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/transport/svg"
)

// Handler は HTTP リクエストルーティングを担う。
type Handler struct {
	cfg         Config
	topicNameOf map[int64]string // topic_id -> name (起動時にロード)
}

// Config はハンドラの構築引数。
type Config struct {
	Templates       *Templates
	Auth            port.AuthService
	Quiz            port.QuizService
	Stats           port.StatsService
	Sets            port.SetRepository
	Questions       port.QuestionRepository
	Topics          port.TopicRepository
	Logger          *slog.Logger
	LoginRateLimit  port.RateLimiter
	GlobalRateLimit port.RateLimiter
	Cookie          CookieConfig
	// StartedAtSecret は started_at の HMAC 署名に使う鍵。32 バイト以上推奨。
	StartedAtSecret []byte
}

// CookieConfig は cookie 名と属性。
type CookieConfig struct {
	SessionName string
	CSRFName    string
	Secure      bool
	Domain      string
}

// NewHandler は Handler を生成する。
//
// cfg.Topics が指定されていれば起動時に topics を 1 度ロードし、id->name の解決に使う。
// 失敗してもアプリは継続 (TopicName が空欄になるだけ)。
func NewHandler(cfg Config) *Handler {
	h := &Handler{cfg: cfg, topicNameOf: map[int64]string{}}
	if cfg.Topics != nil {
		if topics, err := cfg.Topics.ListAll(context.Background()); err == nil {
			for _, t := range topics {
				h.topicNameOf[t.ID] = t.Name
			}
		} else if cfg.Logger != nil {
			cfg.Logger.Error("preload topics", "err", err)
		}
	}
	return h
}

// signStartedAt は (questionID, ms) を HMAC-SHA256 して hex で返す。
func (h *Handler) signStartedAt(questionID, ms int64) string {
	mac := hmac.New(sha256.New, h.cfg.StartedAtSecret)
	_, _ = mac.Write([]byte(strconv.FormatInt(questionID, 10) + ":" + strconv.FormatInt(ms, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyStartedAt は started_at と署名の組を検証する。窓 [now-5min, now+10sec] 外も拒否する。
func (h *Handler) verifyStartedAt(questionID, ms int64, sig string, now time.Time) bool {
	expected := h.signStartedAt(questionID, ms)
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return false
	}
	delta := now.UnixMilli() - ms
	const maxAgeMs = int64(5 * 60 * 1000)  // 5 分
	const futureDriftMs = int64(10 * 1000) // 10 秒
	if delta < -futureDriftMs || delta > maxAgeMs {
		// 過去 5 分以内、未来 10 秒以内のみ許容 (時計ドリフト想定)
		return false
	}
	return true
}

// Register はルートを mux へ登録する。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.home)
	mux.HandleFunc("GET /login", h.getLogin)
	mux.HandleFunc("POST /login", h.postLogin)
	mux.HandleFunc("GET /register", h.getRegister)
	mux.HandleFunc("POST /register", h.postRegister)

	// authenticated
	mux.Handle("GET /quiz", h.authed(http.HandlerFunc(h.getQuiz)))
	mux.Handle("POST /quiz/answer", h.authed(http.HandlerFunc(h.postQuizAnswer)))
	mux.Handle("GET /progress", h.authed(http.HandlerFunc(h.getProgress)))
	mux.Handle("GET /history", h.authed(http.HandlerFunc(h.getHistory)))
	mux.Handle("POST /history/clear", h.authed(http.HandlerFunc(h.postHistoryClear)))
	mux.Handle("POST /history/{id}/delete", h.authed(http.HandlerFunc(h.postHistoryDelete)))
	mux.Handle("GET /settings", h.authed(http.HandlerFunc(h.getSettings)))
	mux.Handle("POST /settings/password", h.authed(http.HandlerFunc(h.postSettingsPassword)))
	mux.Handle("POST /logout", h.authed(http.HandlerFunc(h.postLogout)))
}

func (h *Handler) authed(inner http.Handler) http.Handler {
	mw := middleware.Session(h.cfg.Auth, h.cfg.Cookie.SessionName, "/login")
	csrf := middleware.CSRF(h.cfg.Cookie.CSRFName)
	return mw(csrf(inner))
}

// view は共通テンプレートデータ。
type view struct {
	Title      string
	User       *domain.User
	CSRFToken  string
	CSPNonce   string
	FlashError string
	FlashOK    string
	Summary    *domain.StatsSummary
	// quiz
	Sets         []domain.QuestionSet
	ActiveSet    string
	Mode         string
	Question     *domain.Question
	TopicName    string
	StartedAtMs  int64
	StartedAtSig string // started_at の HMAC-SHA256 (改ざん検知)
	// answer
	Correct     bool
	Explanation string
	NextDueAt   time.Time
	// progress
	DailyChartSVG template.HTML
	TopicBarsSVG  template.HTML
	// history
	Attempts []domain.Attempt
}

func (h *Handler) baseView(r *http.Request, title string) view {
	v := view{Title: title}
	v.User = middleware.UserFrom(r.Context())
	v.CSPNonce = middleware.CSPNonceFrom(r.Context())
	if s := middleware.SessionFrom(r.Context()); s != nil {
		v.CSRFToken = s.CSRFToken
	}
	return v
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	v := h.baseView(r, "ホーム")
	if v.User != nil {
		if summary, err := h.cfg.Stats.Summary(r.Context(), v.User.ID); err == nil {
			v.Summary = &summary
		}
	}
	h.render(w, "home", v)
}

func (h *Handler) getLogin(w http.ResponseWriter, r *http.Request) {
	v := h.baseView(r, "ログイン")
	v.CSRFToken = h.ensureCSRFCookie(w, r)
	h.render(w, "login", v)
}

func (h *Handler) postLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	pwd := r.FormValue("password")
	ip := clientIP(r)
	if h.cfg.LoginRateLimit != nil && !h.cfg.LoginRateLimit.Allow(username+"|"+ip) {
		v := h.baseView(r, "ログイン")
		v.FlashError = "試行回数が多すぎます。少し時間を置いてお試しください。"
		h.render(w, "login", v)
		return
	}
	sess, err := h.cfg.Auth.Login(r.Context(), username, pwd, r.UserAgent(), ip)
	if err != nil {
		v := h.baseView(r, "ログイン")
		v.FlashError = "ユーザー名またはパスワードが違います。"
		h.render(w, "login", v)
		return
	}
	h.setSessionCookies(w, sess)
	http.Redirect(w, r, "/quiz", http.StatusSeeOther)
}

func (h *Handler) getRegister(w http.ResponseWriter, r *http.Request) {
	v := h.baseView(r, "新規登録")
	v.CSRFToken = h.ensureCSRFCookie(w, r)
	h.render(w, "register", v)
}

func (h *Handler) postRegister(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	pwd := r.FormValue("password")
	if _, err := h.cfg.Auth.Register(r.Context(), username, pwd); err != nil {
		v := h.baseView(r, "新規登録")
		v.FlashError = registerErrorMessage(err)
		h.render(w, "register", v)
		return
	}
	sess, err := h.cfg.Auth.Login(r.Context(), username, pwd, r.UserAgent(), clientIP(r))
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	h.setSessionCookies(w, sess)
	http.Redirect(w, r, "/quiz", http.StatusSeeOther)
}

func registerErrorMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrUsernameInvalid):
		return "ユーザー名の形式が不正です (3〜32 文字、英数 _ -)。"
	case errors.Is(err, domain.ErrPasswordTooWeak):
		return "パスワードは 12 文字以上で、英大小数記号のうち3種以上を含めてください。"
	case errors.Is(err, domain.ErrAlreadyExists):
		return "そのユーザー名は既に使われています。"
	}
	return "登録に失敗しました。"
}

func (h *Handler) postLogout(w http.ResponseWriter, r *http.Request) {
	sess := middleware.SessionFrom(r.Context())
	if sess != nil {
		_ = h.cfg.Auth.Logout(r.Context(), sess.ID)
	}
	h.clearSessionCookies(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) getQuiz(w http.ResponseWriter, r *http.Request) {
	v := h.baseView(r, "学習")
	user := middleware.UserFrom(r.Context())
	q := r.URL.Query()
	setCode := q.Get("set")
	if setCode == "" {
		setCode = domain.SetCodeCore
	}
	mode := q.Get("mode")
	if mode == "" {
		mode = "srs"
	}
	sets, err := h.cfg.Sets.ListAll(r.Context())
	if err != nil {
		h.cfg.Logger.Error("sets list failed", "err", err)
	}
	v.Sets = sets
	v.ActiveSet = setCode
	v.Mode = mode

	question, err := h.cfg.Quiz.NextQuestion(r.Context(), user.ID, setCode, domain.QuizMode(mode))
	if err == nil {
		v.Question = question
		v.TopicName = h.topicNameOf[question.TopicID]
		v.StartedAtMs = time.Now().UnixMilli()
		v.StartedAtSig = h.signStartedAt(question.ID, v.StartedAtMs)
	}
	if summary, err := h.cfg.Stats.Summary(r.Context(), user.ID); err == nil {
		v.Summary = &summary
	}
	h.render(w, "quiz", v)
}

func (h *Handler) postQuizAnswer(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	qid, _ := strconv.ParseInt(r.FormValue("question_id"), 10, 64)
	setCode := r.FormValue("set")
	startedAt, _ := strconv.ParseInt(r.FormValue("started_at"), 10, 64)
	startedSig := r.FormValue("started_sig")
	now := time.Now()
	dur := max(int(now.UnixMilli()-startedAt), 0)
	if !h.verifyStartedAt(qid, startedAt, startedSig, now) {
		// 改ざんまたは古すぎる/未来すぎる: SRS grading が歪まないよう中央値 (8s) として扱う
		dur = 8000
	}
	question, err := h.cfg.Questions.GetByID(r.Context(), qid)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	answer := buildAnswerFromForm(r, question.QuestionType)
	in := domain.SubmitInput{
		QuestionID: qid,
		SetCode:    setCode,
		Answer:     answer,
		DurationMs: dur,
	}
	g, err := h.cfg.Quiz.Submit(r.Context(), user.ID, in)
	if err != nil {
		h.cfg.Logger.Error("submit failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	v := h.baseView(r, "採点結果")
	v.ActiveSet = setCode
	v.Mode = r.FormValue("mode")
	v.Correct = g.IsCorrect()
	v.Explanation = g.Explanation
	v.NextDueAt = g.NextDueAt
	h.render(w, "answer", v)
}

func buildAnswerFromForm(r *http.Request, qt domain.QuestionType) domain.AnswerPayload {
	if qt == domain.QuestionTypeJournal {
		debits := readEntries(r, "debit_account_", "debit_amount_", 4)
		credits := readEntries(r, "credit_account_", "credit_amount_", 4)
		return domain.AnswerPayload{Type: qt, Debits: debits, Credits: credits}
	}
	return domain.AnswerPayload{Type: qt, Choice: strings.TrimSpace(r.FormValue("choice"))}
}

func readEntries(r *http.Request, accountPrefix, amountPrefix string, maxRows int) []domain.JournalEntry {
	out := make([]domain.JournalEntry, 0, maxRows)
	for i := 1; i <= maxRows; i++ {
		idx := strconv.Itoa(i)
		acc := strings.TrimSpace(r.FormValue(accountPrefix + idx))
		amt, _ := strconv.ParseInt(r.FormValue(amountPrefix+idx), 10, 64)
		if acc == "" && amt == 0 {
			continue
		}
		out = append(out, domain.JournalEntry{Account: acc, Amount: amt})
	}
	return out
}

func (h *Handler) getProgress(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	v := h.baseView(r, "進捗")
	daily, err := h.cfg.Stats.DailyAccuracy(r.Context(), user.ID, 30)
	if err != nil {
		h.cfg.Logger.Error("daily accuracy", "err", err)
	}
	topics, err := h.cfg.Stats.TopicStats(r.Context(), user.ID)
	if err != nil {
		h.cfg.Logger.Error("topic stats", "err", err)
	}
	v.DailyChartSVG = template.HTML(svg.DailyAccuracyChart(daily, 720, 240)) //nolint:gosec // SVG はサーバ生成、ユーザ入力はエスケープ済み
	v.TopicBarsSVG = template.HTML(svg.TopicAccuracyBars(topics, 720))       //nolint:gosec // 同上
	h.render(w, "progress", v)
}

func (h *Handler) getHistory(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	v := h.baseView(r, "履歴")
	attempts, err := h.cfg.Quiz.History(r.Context(), user.ID, 100, 0)
	if err != nil {
		h.cfg.Logger.Error("history", "err", err)
	}
	v.Attempts = attempts
	h.render(w, "history", v)
}

func (h *Handler) postHistoryClear(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	if err := h.cfg.Quiz.DeleteAllForUser(r.Context(), user.ID); err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/history", http.StatusSeeOther)
}

func (h *Handler) postHistoryDelete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.cfg.Quiz.DeleteAttempt(r.Context(), user.ID, id); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	http.Redirect(w, r, "/history", http.StatusSeeOther)
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	v := h.baseView(r, "設定")
	h.render(w, "settings", v)
}

func (h *Handler) postSettingsPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	user := middleware.UserFrom(r.Context())
	sess := middleware.SessionFrom(r.Context())
	cur := r.FormValue("current")
	newPw := r.FormValue("new")
	err := h.cfg.Auth.ChangePassword(r.Context(), user.ID, cur, newPw, sess.ID)
	v := h.baseView(r, "設定")
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrPasswordMismatch):
			v.FlashError = "現在のパスワードが違います。"
		case errors.Is(err, domain.ErrPasswordTooWeak):
			v.FlashError = "パスワードは 12 文字以上で、英大小数記号のうち3種以上を含めてください。"
		default:
			v.FlashError = "変更に失敗しました。"
		}
	} else {
		v.FlashOK = "パスワードを変更しました。他デバイスのセッションは無効になっています。"
	}
	h.render(w, "settings", v)
}

func (h *Handler) render(w http.ResponseWriter, name string, v view) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.cfg.Templates.Render(w, name, v); err != nil {
		h.cfg.Logger.Error("render", "name", name, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// ensureCSRFCookie は CSRF cookie を初期化する (未ログイン状態用)。
// ログイン後はセッションの CSRFToken が cookie へ書き戻される。
func (h *Handler) ensureCSRFCookie(w http.ResponseWriter, r *http.Request) string {
	c, err := r.Cookie(h.cfg.Cookie.CSRFName)
	if err == nil && len(c.Value) >= 32 {
		return c.Value
	}
	token := randomToken(32)
	h.setCookie(w, h.cfg.Cookie.CSRFName, token, false /* not HttpOnly: JS で読み再送 */)
	return token
}

func (h *Handler) setSessionCookies(w http.ResponseWriter, s *domain.Session) {
	h.setCookie(w, h.cfg.Cookie.SessionName, s.ID, true)
	h.setCookie(w, h.cfg.Cookie.CSRFName, s.CSRFToken, false)
}

func (h *Handler) clearSessionCookies(w http.ResponseWriter) {
	// Secure は BOKI3_COOKIE_SECURE で本番 ON / dev OFF を切り替える。gosec G124 は静的検査で動的判定不可のため抑止。
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure は実行時に Cookie.Secure で制御
		Name: h.cfg.Cookie.SessionName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: h.cfg.Cookie.Secure, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // CSRF cookie は JS から読む必要があり HttpOnly false 固定
		Name: h.cfg.Cookie.CSRFName, Value: "", Path: "/", MaxAge: -1,
		Secure: h.cfg.Cookie.Secure, SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) setCookie(w http.ResponseWriter, name, value string, httpOnly bool) {
	// セッション Cookie は常に HttpOnly=true を強制する。
	// (CSRF Cookie は JS から読む要件があるため呼出側指定を維持)
	effectiveHTTPOnly := httpOnly || name == h.cfg.Cookie.SessionName

	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure は実行時に Cookie.Secure で制御、Session は HttpOnly 強制
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: effectiveHTTPOnly,
		Secure:   h.cfg.Cookie.Secure,
		SameSite: http.SameSiteLaxMode,
		Domain:   h.cfg.Cookie.Domain,
	})
}

// randomToken は CSPRNG から byteLen バイトを取り hex 文字列で返す。
// crypto/rand.Read が失敗するのは OS の CSPRNG が利用不能な極端な状況だけで、
// このアプリは認証に必須のため、その状況では速やかに panic させ呼出側 (Recover middleware) が 500 を返す。
func randomToken(byteLen int) string {
	b := make([]byte, byteLen)
	if _, err := cryptorand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i > 0 {
			v = v[:i]
		}
		return strings.TrimSpace(v)
	}
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return v
	}
	if i := strings.LastIndexByte(r.RemoteAddr, ':'); i > 0 {
		return r.RemoteAddr[:i]
	}
	return r.RemoteAddr
}
