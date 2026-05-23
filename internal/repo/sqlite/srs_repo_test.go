package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/srs"
	repo "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
)

func TestSRSRepoUpsertAndGet(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	uid := insertTestUser(t, db, "u")
	tid := insertTestTopic(t, db, "cash", "現金", 1)
	qid := insertTestQuestion(t, db, "q1", tid)
	r := repo.NewSRSStateRepo(db)
	now := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	s := &srs.State{
		UserID: uid, QuestionID: qid,
		EFactor: 2.5, IntervalDays: 6, Repetitions: 2,
		DueAt: now.AddDate(0, 0, 6), LastGrade: srs.GradeGood, UpdatedAt: now,
	}
	if err := r.Upsert(context.Background(), s); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	// 2 度目で上書き
	s.IntervalDays = 15
	if err := r.Upsert(context.Background(), s); err != nil {
		t.Fatalf("Upsert again: %v", err)
	}
	got, err := r.Get(context.Background(), uid, qid)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.IntervalDays != 15 {
		t.Fatalf("IntervalDays = %d, want 15", got.IntervalDays)
	}
}

func TestSRSRepoDueAndCount(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	uid := insertTestUser(t, db, "u")
	tid := insertTestTopic(t, db, "cash", "現金", 1)
	q1 := insertTestQuestion(t, db, "q1", tid)
	q2 := insertTestQuestion(t, db, "q2", tid)
	r := repo.NewSRSStateRepo(db)
	now := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	if err := r.Upsert(context.Background(), &srs.State{
		UserID: uid, QuestionID: q1, EFactor: 2.5, DueAt: now.AddDate(0, 0, -1), UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert q1: %v", err)
	}
	if err := r.Upsert(context.Background(), &srs.State{
		UserID: uid, QuestionID: q2, EFactor: 2.5, DueAt: now.AddDate(0, 0, 10), UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert q2: %v", err)
	}
	due, err := r.DueForUser(context.Background(), uid, now, 100)
	if err != nil {
		t.Fatalf("DueForUser: %v", err)
	}
	if len(due) != 1 || due[0].QuestionID != q1 {
		t.Fatalf("due = %+v", due)
	}
	n, err := r.CountDueForUser(context.Background(), uid, now)
	if err != nil {
		t.Fatalf("CountDueForUser: %v", err)
	}
	if n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
}

func TestSRSRepoGetNotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewSRSStateRepo(db)
	_, err := r.Get(context.Background(), 1, 1)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want NotFound", err)
	}
}

func TestSRSRepoDeleteAllForUser(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	uid := insertTestUser(t, db, "u")
	tid := insertTestTopic(t, db, "cash", "現金", 1)
	qid := insertTestQuestion(t, db, "q1", tid)
	r := repo.NewSRSStateRepo(db)
	now := time.Now().UTC()
	if err := r.Upsert(context.Background(), &srs.State{
		UserID: uid, QuestionID: qid, EFactor: 2.5, DueAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := r.DeleteAllForUser(context.Background(), uid); err != nil {
		t.Fatalf("DeleteAllForUser: %v", err)
	}
	if _, err := r.Get(context.Background(), uid, qid); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("after delete = %v, want NotFound", err)
	}
}
