package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/idgen"
	jwtauth "github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/jwt"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/clock"
	repo "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/service"
)

var apiTestSecret = []byte("0123456789abcdef0123456789abcdef")

func TestAPIAuthVerifyTokenReturnsSubject(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	clk := clock.Fixed{T: time.Unix(1_700_000_000, 0).UTC()}
	signer, err := jwtauth.NewHS256(apiTestSecret, clk)
	if err != nil {
		t.Fatalf("NewHS256: %v", err)
	}
	svc := service.NewAPIAuthService(signer, repo.NewJWTRevocationRepo(db), idgen.New(), clk, "boki3-quiz", "api")

	token, _, _, err := svc.IssueToken(context.Background(), 42, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	uid, err := svc.VerifyToken(context.Background(), token)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if uid != 42 {
		t.Fatalf("subject = %d, want 42", uid)
	}
}

func TestAPIAuthVerifyTokenRejectsExpired(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	issueClk := clock.Fixed{T: time.Unix(1_700_000_000, 0).UTC()}
	signer, err := jwtauth.NewHS256(apiTestSecret, issueClk)
	if err != nil {
		t.Fatalf("NewHS256: %v", err)
	}
	issuer := service.NewAPIAuthService(signer, repo.NewJWTRevocationRepo(db), idgen.New(), issueClk, "boki3-quiz", "api")
	token, _, _, err := issuer.IssueToken(context.Background(), 1, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	// 検証側だけ 2 時間進んだ clock を使う (署名検証は通し、期限だけ切らす)。
	lateClk := clock.Fixed{T: issueClk.T.Add(2 * time.Hour)}
	verifier := service.NewAPIAuthService(signer, repo.NewJWTRevocationRepo(db), idgen.New(), lateClk, "boki3-quiz", "api")
	_, err = verifier.VerifyToken(context.Background(), token)
	if !errors.Is(err, domain.ErrTokenExpired) {
		t.Fatalf("err = %v, want ErrTokenExpired", err)
	}
}

func TestAPIAuthVerifyTokenRejectsIssuerMismatch(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	clk := clock.Fixed{T: time.Unix(1_700_000_000, 0).UTC()}
	signer, err := jwtauth.NewHS256(apiTestSecret, clk)
	if err != nil {
		t.Fatalf("NewHS256: %v", err)
	}
	issuer := service.NewAPIAuthService(signer, repo.NewJWTRevocationRepo(db), idgen.New(), clk, "boki3-quiz", "api")
	token, _, _, err := issuer.IssueToken(context.Background(), 1, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	other := service.NewAPIAuthService(signer, repo.NewJWTRevocationRepo(db), idgen.New(), clk, "other-app", "api")
	_, err = other.VerifyToken(context.Background(), token)
	if !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("err = %v, want ErrTokenInvalid", err)
	}
}

func TestAPIAuthVerifyTokenRejectsRevoked(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	clk := clock.Fixed{T: time.Unix(1_700_000_000, 0).UTC()}
	signer, err := jwtauth.NewHS256(apiTestSecret, clk)
	if err != nil {
		t.Fatalf("NewHS256: %v", err)
	}
	svc := service.NewAPIAuthService(signer, repo.NewJWTRevocationRepo(db), idgen.New(), clk, "boki3-quiz", "api")
	token, jti, expiresAt, err := svc.IssueToken(context.Background(), 1, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if err := svc.Revoke(context.Background(), jti, 1, expiresAt); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	_, err = svc.VerifyToken(context.Background(), token)
	if !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("err = %v, want ErrTokenInvalid (revoked)", err)
	}
}

func TestAPIAuthVerifyTokenRejectsGarbage(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	clk := clock.Fixed{T: time.Unix(1_700_000_000, 0).UTC()}
	signer, err := jwtauth.NewHS256(apiTestSecret, clk)
	if err != nil {
		t.Fatalf("NewHS256: %v", err)
	}
	svc := service.NewAPIAuthService(signer, repo.NewJWTRevocationRepo(db), idgen.New(), clk, "boki3-quiz", "api")
	_, err = svc.VerifyToken(context.Background(), "not-a-jwt")
	if !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("err = %v, want ErrTokenInvalid", err)
	}
}
