package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	repo "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
)

func TestAttemptRepoCreateAndList(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	uid := insertTestUser(t, db, "u1")
	tid := insertTestTopic(t, db, "cash", "現金", 1)
	qid := insertTestQuestion(t, db, "q1", tid)
	r := repo.NewAttemptRepo(db)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	a := &domain.Attempt{
		UserID:              uid,
		QuestionID:          qid,
		IsCorrect:           true,
		DurationMs:          3000,
		SubmittedAnswerJSON: `{}`,
		AnsweredAt:          now,
	}
	if err := r.Create(context.Background(), a); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID == 0 {
		t.Fatalf("ID not set")
	}
	list, err := r.ListByUser(context.Background(), uid, 10, 0)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 1 || !list[0].IsCorrect {
		t.Fatalf("unexpected list: %+v", list)
	}
}

func TestAttemptRepoDeleteByIDOwnership(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	a := insertTestUser(t, db, "alice")
	b := insertTestUser(t, db, "bob")
	tid := insertTestTopic(t, db, "cash", "現金", 1)
	qid := insertTestQuestion(t, db, "q1", tid)
	r := repo.NewAttemptRepo(db)
	att := &domain.Attempt{UserID: a, QuestionID: qid, IsCorrect: true, DurationMs: 1}
	if err := r.Create(context.Background(), att); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// 他人が削除しようとすると ErrNotFound
	if err := r.DeleteByID(context.Background(), b, att.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other-user delete = %v, want NotFound", err)
	}
	// 本人なら成功
	if err := r.DeleteByID(context.Background(), a, att.ID); err != nil {
		t.Fatalf("self delete: %v", err)
	}
}

func TestAttemptRepoSummaryAndStats(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	uid := insertTestUser(t, db, "u")
	cash := insertTestTopic(t, db, "cash", "現金", 1)
	rec := insertTestTopic(t, db, "receivable", "売掛金", 2)
	qCash := insertTestQuestion(t, db, "qc", cash)
	qRec := insertTestQuestion(t, db, "qr", rec)
	r := repo.NewAttemptRepo(db)
	for _, ok := range []bool{true, false, true} {
		if err := r.Create(context.Background(), &domain.Attempt{
			UserID: uid, QuestionID: qCash, IsCorrect: ok, DurationMs: 1,
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	if err := r.Create(context.Background(), &domain.Attempt{
		UserID: uid, QuestionID: qRec, IsCorrect: true, DurationMs: 1,
	}); err != nil {
		t.Fatalf("Create rec: %v", err)
	}
	total, correct, err := r.SummaryForUser(context.Background(), uid)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if total != 4 || correct != 3 {
		t.Fatalf("summary = (%d,%d), want (4,3)", total, correct)
	}
	stats, err := r.StatsByTopic(context.Background(), uid)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	want := map[string][2]int{"cash": {3, 2}, "receivable": {1, 1}}
	for _, s := range stats {
		w, ok := want[s.TopicCode]
		if !ok {
			continue
		}
		if s.Total != w[0] || s.Correct != w[1] {
			t.Fatalf("topic %s = (%d,%d), want (%d,%d)", s.TopicCode, s.Total, s.Correct, w[0], w[1])
		}
	}
}

func TestAttemptRepoDailyAccuracy(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	uid := insertTestUser(t, db, "u")
	tid := insertTestTopic(t, db, "cash", "現金", 1)
	qid := insertTestQuestion(t, db, "q", tid)
	r := repo.NewAttemptRepo(db)
	day1 := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	for _, ts := range []time.Time{day1, day1, day2} {
		if err := r.Create(context.Background(), &domain.Attempt{
			UserID: uid, QuestionID: qid, IsCorrect: true, DurationMs: 1, AnsweredAt: ts,
		}); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	now := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	got, err := r.DailyAccuracy(context.Background(), uid, 7, now)
	if err != nil {
		t.Fatalf("Daily: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}
