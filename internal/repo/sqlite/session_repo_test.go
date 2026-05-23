package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	repo "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
)

func newSession(id string, userID int64, expires time.Time) *domain.Session {
	return &domain.Session{
		ID:        id,
		UserID:    userID,
		CSRFToken: "csrf-" + id,
		UserAgent: "ua",
		IP:        "127.0.0.1",
		ExpiresAt: expires,
	}
}

func TestSessionRepoCRUD(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	uid := insertTestUser(t, db, "u1")
	r := repo.NewSessionRepo(db)
	exp := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	s := newSession("s1", uid, exp)
	if err := r.Create(context.Background(), s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := r.FindByID(context.Background(), "s1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.UserID != uid || got.CSRFToken != "csrf-s1" {
		t.Fatalf("unexpected session %+v", got)
	}

	if err := r.Touch(context.Background(), "s1", time.Unix(123456789, 0).UTC()); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	got2, _ := r.FindByID(context.Background(), "s1")
	if got2.LastSeenAt.Unix() != 123456789 {
		t.Fatalf("Touch did not update last_seen")
	}

	if err := r.Delete(context.Background(), "s1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := r.FindByID(context.Background(), "s1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("after delete err = %v, want NotFound", err)
	}
}

func TestSessionRepoDeleteAllForUserExcept(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	uid := insertTestUser(t, db, "u2")
	r := repo.NewSessionRepo(db)
	exp := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{"a", "b", "c"} {
		if err := r.Create(context.Background(), newSession(id, uid, exp)); err != nil {
			t.Fatalf("Create %s: %v", id, err)
		}
	}
	if err := r.DeleteAllForUserExcept(context.Background(), uid, "b"); err != nil {
		t.Fatalf("DeleteAllForUserExcept: %v", err)
	}
	if _, err := r.FindByID(context.Background(), "b"); err != nil {
		t.Fatalf("b should remain: %v", err)
	}
	for _, gone := range []string{"a", "c"} {
		if _, err := r.FindByID(context.Background(), gone); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("%s should be gone, got %v", gone, err)
		}
	}
}

func TestSessionRepoPurgeExpired(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	uid := insertTestUser(t, db, "u3")
	r := repo.NewSessionRepo(db)
	past := time.Unix(100, 0).UTC()
	future := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := r.Create(context.Background(), newSession("old", uid, past)); err != nil {
		t.Fatalf("create old: %v", err)
	}
	if err := r.Create(context.Background(), newSession("new", uid, future)); err != nil {
		t.Fatalf("create new: %v", err)
	}
	n, err := r.PurgeExpired(context.Background(), time.Unix(200, 0).UTC())
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if n != 1 {
		t.Fatalf("purged = %d, want 1", n)
	}
	if _, err := r.FindByID(context.Background(), "old"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("old should be gone")
	}
}

func TestSessionRepoTouchNotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewSessionRepo(db)
	err := r.Touch(context.Background(), "no-such", time.Now())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want NotFound", err)
	}
}
