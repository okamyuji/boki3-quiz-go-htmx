// Package middleware は HTTP ミドルウェアを提供する。
//
// 設計方針:
//   - 各ミドルウェアは http.Handler を受け取り http.Handler を返すデコレータ。
//   - リクエストスコープの値は context.Context に格納する。
//   - すべてのミドルウェアはステートレスに使えるよう、必要な依存はクロージャで束ねる。
package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeySession
	ctxKeyUser
	ctxKeyJWTUserID
	ctxKeyCSPNonce
)

// RequestIDFrom は ctx から request ID を取り出す。なければ "".
func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

// SessionFrom は ctx から session を取り出す。
func SessionFrom(ctx context.Context) *domain.Session {
	v, _ := ctx.Value(ctxKeySession).(*domain.Session)
	return v
}

// UserFrom は ctx から user を取り出す。
func UserFrom(ctx context.Context) *domain.User {
	v, _ := ctx.Value(ctxKeyUser).(*domain.User)
	return v
}

// JWTUserIDFrom は ctx から JWT 認証済みの userID を取り出す。なければ 0。
func JWTUserIDFrom(ctx context.Context) int64 {
	v, _ := ctx.Value(ctxKeyJWTUserID).(int64)
	return v
}

// CSPNonceFrom は ctx から CSP nonce を取り出す。
func CSPNonceFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyCSPNonce).(string)
	return v
}

// Recover は panic を捕捉し 500 を返す。
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						"err", rec,
						"path", r.URL.Path,
						"stack", string(debug.Stack()),
					)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestID は X-Request-ID ヘッダから値を取り、無ければ新規生成し ctx に積む。
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" || len(id) > 64 {
				b := make([]byte, 16)
				if _, err := rand.Read(b); err == nil {
					id = hex.EncodeToString(b)
				} else {
					id = "unknown"
				}
			}
			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SecurityHeaders は HSTS / CSP nonce / X-Frame-Options / 等を設定する。
//
// logger が nil の場合は slog.Default を使う。
func SecurityHeaders(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nonceBytes := make([]byte, 16)
			if _, err := rand.Read(nonceBytes); err != nil {
				logger.Error("csp nonce rand", "err", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			nonce := hex.EncodeToString(nonceBytes)
			ctx := context.WithValue(r.Context(), ctxKeyCSPNonce, nonce)

			csp := "default-src 'self'; " +
				"script-src 'self' 'nonce-" + nonce + "'; " +
				"style-src 'self' 'unsafe-inline'; " +
				"img-src 'self' data:; " +
				"font-src 'self'; " +
				"connect-src 'self'; " +
				"frame-ancestors 'none'; " +
				"base-uri 'self'; " +
				"object-src 'none'"
			w.Header().Set("Content-Security-Policy", csp)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// BodyLimit は Request.Body サイズを最大 maxBytes に制限する (DoS 対策)。
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// CORSConfig は CORS 設定。
type CORSConfig struct {
	AllowedOrigins []string // 完全一致のオリジン (例 "https://app.example.com")
	AllowedMethods []string // ["GET","POST",...] 既定 GET/POST
	AllowedHeaders []string // 既定 Authorization, Content-Type, X-CSRF-Token, X-Requested-With
	MaxAge         int      // preflight キャッシュ秒数
	AllowCreds     bool
}

// CORS は CORSConfig に従い CORS ヘッダを設定する。
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	allowed := map[string]struct{}{}
	for _, o := range cfg.AllowedOrigins {
		allowed[o] = struct{}{}
	}
	methods := strings.Join(defaulted(cfg.AllowedMethods, []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}), ", ")
	headers := strings.Join(defaulted(cfg.AllowedHeaders, []string{"Authorization", "Content-Type", "X-CSRF-Token", "X-Requested-With"}), ", ")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					if cfg.AllowCreds {
						w.Header().Set("Access-Control-Allow-Credentials", "true")
					}
					w.Header().Set("Access-Control-Allow-Methods", methods)
					w.Header().Set("Access-Control-Allow-Headers", headers)
					if cfg.MaxAge > 0 {
						w.Header().Set("Access-Control-Max-Age", intToASCII(cfg.MaxAge))
					}
				}
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitByIP は RemoteAddr ベースのレートリミットを適用する。
func RateLimitByIP(rl port.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !rl.Allow(ip) {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Session は session_id Cookie からセッションをロードし ctx に積む。
// 認証必須ハンドラの前段で使う。失敗時は redirect (HTML) または 401 (JSON) を返す。
func Session(auth port.AuthService, cookieName string, htmlLoginPath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(cookieName)
			if err != nil || c.Value == "" {
				redirectOrJSON(w, r, htmlLoginPath, "unauthenticated", http.StatusUnauthorized)
				return
			}
			sess, user, err := auth.SessionByID(r.Context(), c.Value)
			if err != nil {
				redirectOrJSON(w, r, htmlLoginPath, "unauthenticated", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeySession, sess)
			ctx = context.WithValue(ctx, ctxKeyUser, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CSRF は double-submit cookie + server-stored の両方を検証する。
// 安全メソッド (GET/HEAD/OPTIONS) は素通り。それ以外は X-CSRF-Token か form の csrf_token と
// セッションの CSRFToken が一致することを要求する。
func CSRF(cookieName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			sess := SessionFrom(r.Context())
			if sess == nil {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			submitted := r.Header.Get("X-CSRF-Token")
			if submitted == "" {
				submitted = r.FormValue("csrf_token")
			}
			c, err := r.Cookie(cookieName)
			if err != nil || c.Value == "" {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			if !hmac.Equal([]byte(submitted), []byte(c.Value)) ||
				!hmac.Equal([]byte(c.Value), []byte(sess.CSRFToken)) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// JWTBearer は Authorization: Bearer <token> を検証し、userID を ctx に積む。
// 認証失敗は 401 JSON を返す。
func JWTBearer(api port.APIAuthService) func(http.Handler) http.Handler {
	writeJSONErr := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(auth, prefix) {
				writeJSONErr(w, http.StatusUnauthorized, `{"error":"unauthenticated"}`)
				return
			}
			tok := strings.TrimPrefix(auth, prefix)
			uid, err := api.VerifyToken(r.Context(), tok)
			if err != nil {
				switch {
				case errors.Is(err, domain.ErrTokenExpired):
					writeJSONErr(w, http.StatusUnauthorized, `{"error":"token_expired"}`)
				default:
					writeJSONErr(w, http.StatusUnauthorized, `{"error":"unauthenticated"}`)
				}
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeyJWTUserID, uid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Chain は middlewares を内側 (handler 直接) から外側へ巻く。
//
// 例: Chain(h, A, B, C) は A(B(C(h))) を返す。Recover を最外周に置きたい場合は最初に書く。
func Chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// AccessLog はリクエストを構造化ログに出力する。
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &recordedWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(rw, r)
			logger.Info("http.request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", RequestIDFrom(r.Context()),
				"ip", clientIP(r),
			)
		})
	}
}

type recordedWriter struct {
	http.ResponseWriter
	status int
}

func (r *recordedWriter) WriteHeader(c int) { r.status = c; r.ResponseWriter.WriteHeader(c) }

// --- helpers ---

func isSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
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
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func redirectOrJSON(w http.ResponseWriter, r *http.Request, htmlPath, msg string, status int) {
	if strings.HasPrefix(r.URL.Path, "/api/") || r.Header.Get("Accept") == "application/json" {
		http.Error(w, `{"error":"`+msg+`"}`, status)
		return
	}
	http.Redirect(w, r, htmlPath, http.StatusSeeOther)
}

func defaulted(v, def []string) []string {
	if len(v) == 0 {
		return def
	}
	return v
}

func intToASCII(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
