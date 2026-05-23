package service_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/idgen"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/password"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/clock"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/sqlitex"
	repo "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/service"
)

func newTestServiceDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := sqlitex.Open("file:" + filepath.Join(dir, "svc.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := repo.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func newAuthService(t *testing.T, db *sql.DB) *service.AuthService {
	t.Helper()
	users := repo.NewUserRepo(db)
	sessions := repo.NewSessionRepo(db)
	jwts := repo.NewJWTRevocationRepo(db)
	hasher := password.New(password.Params{N: 1024, R: 8, P: 1, KeyLen: 32, SaltLen: 16})
	g := idgen.New()
	clk := clock.Fixed{T: time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)}
	return service.NewAuthService(users, sessions, jwts, hasher, g, clk, service.DefaultAuthConfig())
}

func TestAuthRegisterAndLogin(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	a := newAuthService(t, db)
	u, err := a.Register(context.Background(), "alice01", "P@ssw0rd!Strong")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if u.ID == 0 {
		t.Fatalf("user ID not set")
	}
	s, err := a.Login(context.Background(), "alice01", "P@ssw0rd!Strong", "ua", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if s.UserID != u.ID {
		t.Fatalf("session UserID mismatch")
	}
}

func TestAuthRegisterRejectsWeakPassword(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	a := newAuthService(t, db)
	_, err := a.Register(context.Background(), "bob1234", "short1")
	if !errors.Is(err, domain.ErrPasswordTooWeak) {
		t.Fatalf("err = %v, want PasswordTooWeak", err)
	}
}

func TestAuthRegisterRejectsInvalidUsername(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	a := newAuthService(t, db)
	_, err := a.Register(context.Background(), "x", "P@ssw0rd!Strong")
	if !errors.Is(err, domain.ErrUsernameInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func TestAuthLoginWrongPassword(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	a := newAuthService(t, db)
	if _, err := a.Register(context.Background(), "carol01", "P@ssw0rd!Strong"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	_, err := a.Login(context.Background(), "carol01", "wrong-password-1", "ua", "ip")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("err = %v, want Unauthorized", err)
	}
}

func TestAuthChangePasswordRevokesOtherSessions(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	a := newAuthService(t, db)
	u, err := a.Register(context.Background(), "dave1234", "P@ssw0rd!Strong")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	s1, _ := a.Login(context.Background(), "dave1234", "P@ssw0rd!Strong", "ua1", "1.1.1.1")
	s2, _ := a.Login(context.Background(), "dave1234", "P@ssw0rd!Strong", "ua2", "2.2.2.2")
	if err := a.ChangePassword(context.Background(), u.ID, "P@ssw0rd!Strong", "NewP@ssw0rd!Strong", s1.ID); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	// s1 は残り、s2 は破棄
	_, _, err = a.SessionByID(context.Background(), s1.ID)
	if err != nil {
		t.Fatalf("s1 should remain: %v", err)
	}
	_, _, err = a.SessionByID(context.Background(), s2.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("s2 expected gone, got %v", err)
	}
}
