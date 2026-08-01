package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

const csrfCookie = "test_csrf"

func csrfHandler() http.Handler {
	mw := CSRF(csrfCookie)
	return mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

func withSession(r *http.Request, token string) *http.Request {
	sess := &domain.Session{ID: "s1", CSRFToken: token}
	return r.WithContext(context.WithValue(r.Context(), ctxKeySession, sess))
}

func doCSRF(t *testing.T, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	csrfHandler().ServeHTTP(rec, r)
	return rec
}

func TestCSRFAllowsSafeMethodWithoutToken(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodGet, "/x", http.NoBody)
	if rec := doCSRF(t, r); rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", rec.Code)
	}
}

func TestCSRFRejectsPostWithoutSession(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/x", http.NoBody)
	if rec := doCSRF(t, r); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestCSRFRejectsPostWithoutCookie(t *testing.T) {
	t.Parallel()
	r := withSession(httptest.NewRequest(http.MethodPost, "/x", http.NoBody), "tok")
	r.Header.Set("X-CSRF-Token", "tok")
	if rec := doCSRF(t, r); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestCSRFAcceptsMatchingHeaderToken(t *testing.T) {
	t.Parallel()
	r := withSession(httptest.NewRequest(http.MethodPost, "/x", http.NoBody), "tok")
	r.Header.Set("X-CSRF-Token", "tok")
	r.AddCookie(&http.Cookie{Name: csrfCookie, Value: "tok"})
	if rec := doCSRF(t, r); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestCSRFAcceptsMatchingFormToken(t *testing.T) {
	t.Parallel()
	form := url.Values{"csrf_token": {"tok"}}
	r := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = withSession(r, "tok")
	r.AddCookie(&http.Cookie{Name: csrfCookie, Value: "tok"})
	if rec := doCSRF(t, r); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestCSRFRejectsTokenCookieMismatch(t *testing.T) {
	t.Parallel()
	r := withSession(httptest.NewRequest(http.MethodPost, "/x", http.NoBody), "tok")
	r.Header.Set("X-CSRF-Token", "different")
	r.AddCookie(&http.Cookie{Name: csrfCookie, Value: "tok"})
	if rec := doCSRF(t, r); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (submitted != cookie)", rec.Code)
	}
}

func TestCSRFRejectsCookieSessionMismatch(t *testing.T) {
	t.Parallel()
	r := withSession(httptest.NewRequest(http.MethodPost, "/x", http.NoBody), "session-tok")
	r.Header.Set("X-CSRF-Token", "cookie-tok")
	r.AddCookie(&http.Cookie{Name: csrfCookie, Value: "cookie-tok"})
	if rec := doCSRF(t, r); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (cookie != session)", rec.Code)
	}
}
