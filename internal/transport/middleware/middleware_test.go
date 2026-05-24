package middleware_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/transport/middleware"
)

func noopHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

func TestRecoverCatchesPanic(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := middleware.Recover(logger)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/x", http.NoBody)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d", w.Code)
	}
}

func TestRequestIDSetsHeader(t *testing.T) {
	t.Parallel()
	var seen string
	h := middleware.RequestID()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = middleware.RequestIDFrom(r.Context())
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", http.NoBody)
	h.ServeHTTP(w, r)
	if seen == "" || w.Header().Get("X-Request-ID") == "" {
		t.Fatalf("X-Request-ID not set")
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()
	h := middleware.SecurityHeaders(nil)(noopHandler())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", http.NoBody)
	h.ServeHTTP(w, r)
	must := []string{"Content-Security-Policy", "X-Frame-Options", "Strict-Transport-Security", "Referrer-Policy"}
	for _, k := range must {
		if w.Header().Get(k) == "" {
			t.Fatalf("missing header %q", k)
		}
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "nonce-") {
		t.Fatalf("CSP missing nonce")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("X-Frame-Options = %q", w.Header().Get("X-Frame-Options"))
	}
}

func TestBodyLimitRejectsLarge(t *testing.T) {
	t.Parallel()
	h := middleware.BodyLimit(8)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		_, _ = w.Write(b)
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader("0123456789"))
	h.ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code = %d, want 413", w.Code)
	}
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	t.Parallel()
	h := middleware.CORS(middleware.CORSConfig{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowCreds:     true,
		MaxAge:         600,
	})(noopHandler())
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Fatalf("Allow-Origin = %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("creds header missing")
	}
}

func TestCORSPreflight(t *testing.T) {
	t.Parallel()
	h := middleware.CORS(middleware.CORSConfig{AllowedOrigins: []string{"https://x.example"}})(noopHandler())
	r := httptest.NewRequest("OPTIONS", "/", http.NoBody)
	r.Header.Set("Origin", "https://x.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight = %d, want 204", w.Code)
	}
}

type fakeRL struct{ allow bool }

func (f fakeRL) Allow(string) bool { return f.allow }

func TestRateLimitByIPBlocks(t *testing.T) {
	t.Parallel()
	h := middleware.RateLimitByIP(fakeRL{allow: false})(noopHandler())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.RemoteAddr = "1.2.3.4:1234"
	h.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("code = %d, want 429", w.Code)
	}
}

func TestChainOrder(t *testing.T) {
	t.Parallel()
	order := ""
	mid := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order += name + ":pre "
				next.ServeHTTP(w, r)
				order += name + ":post "
			})
		}
	}
	final := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { order += "final " })
	h := middleware.Chain(final, mid("a"), mid("b"))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", http.NoBody))
	want := "a:pre b:pre final b:post a:post "
	if order != want {
		t.Fatalf("order = %q, want %q", order, want)
	}
}

// ensure context propagation works
func TestContextProvidersDoNotPanic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if middleware.SessionFrom(ctx) != nil {
		t.Fatalf("expected nil session")
	}
	if middleware.UserFrom(ctx) != nil {
		t.Fatalf("expected nil user")
	}
	if middleware.JWTUserIDFrom(ctx) != 0 {
		t.Fatalf("expected 0 jwt uid")
	}
	if middleware.CSPNonceFrom(ctx) != "" {
		t.Fatalf("expected empty nonce")
	}
}
