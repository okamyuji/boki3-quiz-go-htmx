package sqlite_test

import (
	"context"
	"errors"
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	repo "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
)

func TestSetRepoGetAndList(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	_ = insertTestSet(t, db, "core_300", "コア300", 300)
	_ = insertTestSet(t, db, "journal_240", "仕訳240", 240)
	r := repo.NewSetRepo(db)
	got, err := r.GetByCode(context.Background(), "core_300")
	if err != nil {
		t.Fatalf("GetByCode: %v", err)
	}
	if got.Name != "コア300" || got.TargetSize != 300 {
		t.Fatalf("unexpected %+v", got)
	}
	list, err := r.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
}

func TestSetRepoGetNotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewSetRepo(db)
	_, err := r.GetByCode(context.Background(), "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want NotFound", err)
	}
}
