package jwt_test

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/jwt"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/clock"
)

// testNow は全テストで共有する固定の「現在時刻」。
// signer に clock.Fixed{T: testNow} を注入することで、exp 判定が実行日時に
// 依存しなくなり、時限爆弾化も flaky 化もしない決定的なテストになる。
var testNow = time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)

func newSigner(t *testing.T) *jwt.HS256Signer {
	t.Helper()
	secret := bytes.Repeat([]byte{0xab}, 32)
	s, err := jwt.NewHS256(secret, clock.Fixed{T: testNow})
	if err != nil {
		t.Fatalf("NewHS256: %v", err)
	}
	return s
}

func TestSignAndParse(t *testing.T) {
	t.Parallel()
	s := newSigner(t)
	// 固定クロック (testNow) を基準に exp を未来へ置く。signer も testNow を
	// 現在時刻とみなすため、実行日時に関わらず「期限内」と判定される。
	now := testNow
	claims := domain.JWTClaims{
		Subject: 42, Issuer: "boki3-quiz", Audience: "api",
		IssuedAt: now, ExpiresAt: now.Add(time.Hour), JTI: "abc",
	}
	tok, err := s.Sign(claims)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if strings.Count(tok, ".") != 2 {
		t.Fatalf("token shape: %q", tok)
	}
	got, err := s.Parse(tok)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Subject != 42 || got.JTI != "abc" || got.Issuer != "boki3-quiz" {
		t.Fatalf("claims mismatch: %+v", got)
	}
}

func TestParseRejectsNoneAlg(t *testing.T) {
	t.Parallel()
	s := newSigner(t)
	// alg=none を持つ手作りトークン (署名は空) は拒否される。
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":1}`))
	tok := header + "." + payload + "."
	if _, err := s.Parse(tok); err == nil {
		t.Fatalf("expected alg=none to be rejected")
	}
}

func TestParseRejectsRS256(t *testing.T) {
	t.Parallel()
	s := newSigner(t)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":1}`))
	sig := base64.RawURLEncoding.EncodeToString([]byte("fake"))
	tok := header + "." + payload + "." + sig
	if _, err := s.Parse(tok); err == nil {
		t.Fatalf("expected alg=RS256 to be rejected")
	}
}

func TestParseRejectsBadSignature(t *testing.T) {
	t.Parallel()
	s := newSigner(t)
	claims := domain.JWTClaims{Subject: 1, IssuedAt: testNow, ExpiresAt: testNow.Add(time.Hour), JTI: "j"}
	tok, _ := s.Sign(claims)
	tampered := tok[:len(tok)-2] + "xx"
	if _, err := s.Parse(tampered); err == nil {
		t.Fatalf("expected tampered signature to fail")
	}
}

func TestParseRejectsExpired(t *testing.T) {
	t.Parallel()
	s := newSigner(t)
	// exp を固定クロック (testNow) より過去に置くと、署名は正しくても拒否される。
	// クロック注入により、この期限切れ判定を決定的に検証できる。
	claims := domain.JWTClaims{
		Subject: 7, Issuer: "boki3-quiz", Audience: "api",
		IssuedAt: testNow.Add(-2 * time.Hour), ExpiresAt: testNow.Add(-time.Hour), JTI: "exp",
	}
	tok, err := s.Sign(claims)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, err := s.Parse(tok); err == nil {
		t.Fatalf("expected expired token to be rejected")
	}
}

func TestParseShortSecretRejected(t *testing.T) {
	t.Parallel()
	if _, err := jwt.NewHS256([]byte("short"), clock.Fixed{T: testNow}); err == nil {
		t.Fatalf("expected short secret to be rejected")
	}
}

func TestParseMalformedToken(t *testing.T) {
	t.Parallel()
	s := newSigner(t)
	if _, err := s.Parse("not.a.real.jwt.shape"); err == nil {
		t.Fatalf("expected malformed token to fail")
	}
	if _, err := s.Parse("only.two"); err == nil {
		t.Fatalf("expected 2-segment token to fail")
	}
}
