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

func TestAttemptRepoLastQuestionIDInSet(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	uid := insertTestUser(t, db, "u")
	other := insertTestUser(t, db, "other")
	tid := insertTestTopic(t, db, "cash", "現金", 1)
	q1 := insertTestQuestion(t, db, "q1", tid)
	q2 := insertTestQuestion(t, db, "q2", tid)
	setA := insertTestSet(t, db, "core_300", "コア", 300)
	setB := insertTestSet(t, db, "journal_240", "仕訳", 240)
	insertSetMember(t, db, setA, q1, 1)
	insertSetMember(t, db, setA, q2, 2)
	insertSetMember(t, db, setB, q1, 1)
	r := repo.NewAttemptRepo(db)
	ctx := context.Background()

	if _, err := r.LastQuestionIDInSet(ctx, uid, setA); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("no attempts = %v, want ErrNotFound", err)
	}

	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	// setA: q1 → q2 の順に回答し、その後 setB で q1、最後に他ユーザが setA で q1 を回答。
	for i, a := range []struct {
		user, qid, setID int64
	}{{uid, q1, setA}, {uid, q2, setA}, {uid, q1, setB}, {other, q1, setA}} {
		sid := a.setID
		if err := r.Create(ctx, &domain.Attempt{
			UserID: a.user, QuestionID: a.qid, SetID: &sid,
			IsCorrect: true, DurationMs: 1, SubmittedAnswerJSON: `{}`,
			AnsweredAt: base.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("create attempt %d: %v", i, err)
		}
	}

	got, err := r.LastQuestionIDInSet(ctx, uid, setA)
	if err != nil {
		t.Fatalf("LastQuestionIDInSet(setA): %v", err)
	}
	if got != q2 {
		t.Fatalf("setA last = %d, want %d (直近の setB/他ユーザの回答に影響されない)", got, q2)
	}
	gotB, err := r.LastQuestionIDInSet(ctx, uid, setB)
	if err != nil {
		t.Fatalf("LastQuestionIDInSet(setB): %v", err)
	}
	if gotB != q1 {
		t.Fatalf("setB last = %d, want %d", gotB, q1)
	}
}

func TestAttemptRepoWeakTopicIDs(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	uid := insertTestUser(t, db, "u")
	cash := insertTestTopic(t, db, "cash", "現金", 1)       // 誤答率 1/2 = 50%
	rec := insertTestTopic(t, db, "receivable", "売掛金", 2) // 誤答率 1/1 = 100%
	fixed := insertTestTopic(t, db, "fixed", "固定資産", 3)   // 全問正解 → 対象外
	old := insertTestTopic(t, db, "old", "旧論点", 4)        // 誤答が since より前 → 対象外
	qCash := insertTestQuestion(t, db, "qc", cash)
	qRec := insertTestQuestion(t, db, "qr", rec)
	qFixed := insertTestQuestion(t, db, "qf", fixed)
	qOld := insertTestQuestion(t, db, "qo", old)
	r := repo.NewAttemptRepo(db)
	ctx := context.Background()
	now := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	since := now.AddDate(0, 0, -7)
	for _, a := range []struct {
		qid     int64
		correct bool
		at      time.Time
	}{
		{qCash, true, now.Add(-time.Hour)},
		{qCash, false, now.Add(-2 * time.Hour)},
		{qRec, false, now.Add(-3 * time.Hour)},
		{qFixed, true, now.Add(-4 * time.Hour)},
		{qOld, false, since.Add(-time.Hour)},
	} {
		if err := r.Create(ctx, &domain.Attempt{
			UserID: uid, QuestionID: a.qid, IsCorrect: a.correct,
			DurationMs: 1, SubmittedAnswerJSON: `{}`, AnsweredAt: a.at,
		}); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	got, err := r.WeakTopicIDs(ctx, uid, since, 10)
	if err != nil {
		t.Fatalf("WeakTopicIDs: %v", err)
	}
	want := []int64{rec, cash}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v (誤答率降順、全問正解と期間外は除外)", got, want)
	}

	limited, err := r.WeakTopicIDs(ctx, uid, since, 1)
	if err != nil {
		t.Fatalf("WeakTopicIDs limit: %v", err)
	}
	if len(limited) != 1 || limited[0] != rec {
		t.Fatalf("limit=1 got %v, want [%d]", limited, rec)
	}

	// 境界値: limit 0 以下は既定値 (3 件) に丸められる。
	defaulted, err := r.WeakTopicIDs(ctx, uid, since, 0)
	if err != nil {
		t.Fatalf("WeakTopicIDs limit=0: %v", err)
	}
	if len(defaulted) != 2 {
		t.Fatalf("limit=0 got %d topics, want 2 (既定 3 件以内)", len(defaulted))
	}
}

// 異常系: DB クローズ後は各照会がエラーを返す。
func TestAttemptRepoQueriesFailOnClosedDB(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewAttemptRepo(db)
	_ = db.Close()
	ctx := context.Background()
	if _, err := r.LastQuestionIDInSet(ctx, 1, 1); err == nil || errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("LastQuestionIDInSet on closed db = %v, want non-NotFound error", err)
	}
	if _, err := r.WeakTopicIDs(ctx, 1, time.Unix(0, 0), 3); err == nil {
		t.Fatalf("WeakTopicIDs on closed db: want error")
	}
	if _, err := r.AttemptedQuestionIDs(ctx, 1); err == nil {
		t.Fatalf("AttemptedQuestionIDs on closed db: want error")
	}
}

func TestAttemptRepoAttemptedQuestionIDs(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	uid := insertTestUser(t, db, "u")
	other := insertTestUser(t, db, "other")
	tid := insertTestTopic(t, db, "cash", "現金", 1)
	q1 := insertTestQuestion(t, db, "q1", tid)
	q2 := insertTestQuestion(t, db, "q2", tid)
	q3 := insertTestQuestion(t, db, "q3", tid)
	r := repo.NewAttemptRepo(db)
	ctx := context.Background()
	for _, a := range []struct {
		user, qid int64
	}{{uid, q1}, {uid, q1}, {uid, q2}, {other, q3}} {
		if err := r.Create(ctx, &domain.Attempt{
			UserID: a.user, QuestionID: a.qid, IsCorrect: true,
			DurationMs: 1, SubmittedAnswerJSON: `{}`,
		}); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	got, err := r.AttemptedQuestionIDs(ctx, uid)
	if err != nil {
		t.Fatalf("AttemptedQuestionIDs: %v", err)
	}
	if len(got) != 2 || got[0] != q1 || got[1] != q2 {
		t.Fatalf("got %v, want [%d %d] (重複排除・昇順・他ユーザ除外)", got, q1, q2)
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
