package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	repo "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
)

func TestUserPrefsGetReturnsNotFoundWhenAbsent(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewUserPrefsRepo(db)

	_, err := r.Get(context.Background(), 12345)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get err = %v, want domain.ErrNotFound", err)
	}
}

func TestUserPrefsUpsertInsertsAndGetReturnsSavedValues(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewUserPrefsRepo(db)
	uid := insertTestUser(t, db, "prefs-user-1")

	at := time.Unix(1700000000, 0).UTC()
	in := domain.UserPrefs{UserID: uid, QuizSet: "journal_150", QuizMode: domain.QuizModeRandom, UpdatedAt: at}
	if err := r.Upsert(context.Background(), &in); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := r.Get(context.Background(), uid)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UserID != uid || got.QuizSet != "journal_150" || got.QuizMode != domain.QuizModeRandom {
		t.Fatalf("Get = %+v, want saved values", got)
	}
	if !got.UpdatedAt.Equal(at) {
		t.Fatalf("UpdatedAt = %v, want %v", got.UpdatedAt, at)
	}
}

func TestUserPrefsUpsertUpdatesExistingRow(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewUserPrefsRepo(db)
	uid := insertTestUser(t, db, "prefs-user-2")

	first := domain.UserPrefs{UserID: uid, QuizSet: "core_300", QuizMode: domain.QuizModeSRS, UpdatedAt: time.Unix(1700000000, 0).UTC()}
	if err := r.Upsert(context.Background(), &first); err != nil {
		t.Fatalf("Upsert first: %v", err)
	}
	second := domain.UserPrefs{UserID: uid, QuizSet: "comprehensive_50", QuizMode: domain.QuizModeSequential, UpdatedAt: time.Unix(1700000100, 0).UTC()}
	if err := r.Upsert(context.Background(), &second); err != nil {
		t.Fatalf("Upsert second: %v", err)
	}

	got, err := r.Get(context.Background(), uid)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.QuizSet != "comprehensive_50" || got.QuizMode != domain.QuizModeSequential {
		t.Fatalf("Get after update = %+v, want second values", got)
	}
	if !got.UpdatedAt.Equal(second.UpdatedAt) {
		t.Fatalf("UpdatedAt = %v, want %v", got.UpdatedAt, second.UpdatedAt)
	}

	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM user_prefs WHERE user_id = ?`, uid).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1 (upsert must not duplicate)", count)
	}
}
