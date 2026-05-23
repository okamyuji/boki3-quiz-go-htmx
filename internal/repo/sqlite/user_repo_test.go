package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	repo "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
)

func TestUserRepoCreateAndFind(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewUserRepo(db)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	u := &domain.User{
		Username:          "alice",
		PasswordHash:      []byte("hash"),
		PasswordSalt:      []byte("salt"),
		PasswordParams:    "scrypt$N=32768",
		PasswordUpdatedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := r.Create(context.Background(), u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == 0 {
		t.Fatalf("ID not set")
	}
	got, err := r.FindByUsername(context.Background(), "ALICE") // case-insensitive
	if err != nil {
		t.Fatalf("FindByUsername: %v", err)
	}
	if got.ID != u.ID {
		t.Fatalf("ID mismatch %d vs %d", got.ID, u.ID)
	}
	if !got.CreatedAt.Equal(now) {
		t.Fatalf("CreatedAt mismatch %v vs %v", got.CreatedAt, now)
	}
}

func TestUserRepoCreateDuplicateUsername(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewUserRepo(db)
	u := &domain.User{Username: "bob", PasswordHash: []byte("h"), PasswordSalt: []byte("s"), PasswordParams: "p"}
	if err := r.Create(context.Background(), u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	u2 := &domain.User{Username: "BOB", PasswordHash: []byte("h"), PasswordSalt: []byte("s"), PasswordParams: "p"}
	err := r.Create(context.Background(), u2)
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("err = %v, want ErrAlreadyExists", err)
	}
}

func TestUserRepoFindByUsernameNotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewUserRepo(db)
	_, err := r.FindByUsername(context.Background(), "no-such-user")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUserRepoUpdatePassword(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewUserRepo(db)
	u := &domain.User{Username: "carol", PasswordHash: []byte("old"), PasswordSalt: []byte("s"), PasswordParams: "p"}
	if err := r.Create(context.Background(), u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	newTime := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := r.UpdatePassword(context.Background(), u.ID, []byte("new"), []byte("s2"), "p2", newTime); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	got, err := r.FindByID(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if string(got.PasswordHash) != "new" {
		t.Fatalf("hash = %q, want \"new\"", got.PasswordHash)
	}
	if got.PasswordParams != "p2" {
		t.Fatalf("params = %q", got.PasswordParams)
	}
	if !got.PasswordUpdatedAt.Equal(newTime) {
		t.Fatalf("password_updated_at mismatch")
	}
}

func TestUserRepoUpdatePasswordNotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewUserRepo(db)
	err := r.UpdatePassword(context.Background(), 99999, []byte("x"), []byte("y"), "p", time.Now())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
