package jwt_test

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/jwt"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

func newSigner(t *testing.T) *jwt.HS256Signer {
	t.Helper()
	secret := bytes.Repeat([]byte{0xab}, 32)
	s, err := jwt.NewHS256(secret)
	if err != nil {
		t.Fatalf("NewHS256: %v", err)
	}
	return s
}

func TestSignAndParse(t *testing.T) {
	t.Parallel()
	s := newSigner(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
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
	claims := domain.JWTClaims{Subject: 1, IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), JTI: "j"}
	tok, _ := s.Sign(claims)
	tampered := tok[:len(tok)-2] + "xx"
	if _, err := s.Parse(tampered); err == nil {
		t.Fatalf("expected tampered signature to fail")
	}
}

func TestParseShortSecretRejected(t *testing.T) {
	t.Parallel()
	if _, err := jwt.NewHS256([]byte("short")); err == nil {
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
