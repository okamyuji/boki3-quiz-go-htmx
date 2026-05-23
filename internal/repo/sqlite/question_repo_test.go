package sqlite_test

import (
	"context"
	"errors"
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	repo "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
)

func TestQuestionRepoGetAndList(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	tid := insertTestTopic(t, db, "cash", "現金預金", 1)
	q1 := insertTestQuestion(t, db, "q1", tid)
	q2 := insertTestQuestion(t, db, "q2", tid)
	setID := insertTestSet(t, db, "core_300", "コア300", 300)
	insertSetMember(t, db, setID, q1, 1)
	insertSetMember(t, db, setID, q2, 2)

	r := repo.NewQuestionRepo(db)
	got, err := r.GetByID(context.Background(), q1)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Code != "q1" || got.TopicID != tid {
		t.Fatalf("unexpected %+v", got)
	}
	gotByCode, err := r.GetByCode(context.Background(), "q2")
	if err != nil {
		t.Fatalf("GetByCode: %v", err)
	}
	if gotByCode.ID != q2 {
		t.Fatalf("GetByCode ID = %d", gotByCode.ID)
	}

	list, err := r.ListBySet(context.Background(), "core_300")
	if err != nil {
		t.Fatalf("ListBySet: %v", err)
	}
	if len(list) != 2 || list[0].Code != "q1" || list[1].Code != "q2" {
		t.Fatalf("ListBySet order/content: %+v", list)
	}
}

func TestQuestionRepoSearchByTopic(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	cash := insertTestTopic(t, db, "cash", "現金預金", 1)
	rec := insertTestTopic(t, db, "receivable", "売掛金", 2)
	_ = insertTestQuestion(t, db, "qa", cash)
	_ = insertTestQuestion(t, db, "qb", rec)
	r := repo.NewQuestionRepo(db)
	got, err := r.Search(context.Background(), domain.QuestionFilter{TopicCodes: []string{"cash"}})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Code != "qa" {
		t.Fatalf("got = %+v", got)
	}
}

func TestQuestionRepoGetByIDNotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewQuestionRepo(db)
	_, err := r.GetByID(context.Background(), 9999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want NotFound", err)
	}
}

func TestQuestionRepoSearchLimitOffset(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	tid := insertTestTopic(t, db, "cash", "現金", 1)
	for i := range 5 {
		_ = insertTestQuestion(t, db, "qx"+string(rune('0'+i)), tid)
	}
	r := repo.NewQuestionRepo(db)
	got, err := r.Search(context.Background(), domain.QuestionFilter{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}
