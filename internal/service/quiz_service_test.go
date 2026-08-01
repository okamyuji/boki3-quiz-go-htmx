package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/clock"
	repo "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/service"
)

func seedQuiz(t *testing.T, db *sql.DB) (userID, qID int64) {
	t.Helper()
	ctx := context.Background()
	res, err := db.ExecContext(ctx,
		`INSERT INTO users(username, password_hash, password_salt, password_params, password_updated_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 0, 0, 0)`,
		"u1", []byte("h"), []byte("s"), "scrypt$N=1")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	uid, _ := res.LastInsertId()

	res, err = db.ExecContext(ctx, `INSERT INTO topics(code, name, ord) VALUES (?, ?, ?)`, "cash", "現金", 1)
	if err != nil {
		t.Fatalf("insert topic: %v", err)
	}
	topicID, _ := res.LastInsertId()

	ans := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "現金", Amount: 1000}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 1000}},
	}
	ansJSON, _ := json.Marshal(ans)

	res, err = db.ExecContext(ctx,
		`INSERT INTO questions(code, topic_id, question_type, difficulty, prompt, payload_json, answer_json, explanation, references_json, created_at)
		 VALUES (?, ?, 'journal', 1, ?, '{}', ?, ?, NULL, 0)`,
		"q1", topicID, "現金売上", string(ansJSON), "exp")
	if err != nil {
		t.Fatalf("insert question: %v", err)
	}
	qID, _ = res.LastInsertId()

	res, _ = db.ExecContext(ctx,
		`INSERT INTO question_sets(code, name, description, target_size) VALUES (?, ?, '', ?)`,
		"core_300", "コア300", 300)
	setID, _ := res.LastInsertId()
	_, _ = db.ExecContext(ctx,
		`INSERT INTO question_set_members(set_id, question_id, ord) VALUES (?, ?, 1)`, setID, qID)
	return uid, qID
}

func TestQuizSubmitCorrect(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	uid, qID := seedQuiz(t, db)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	q := service.NewQuizService(
		repo.NewQuestionRepo(db),
		repo.NewSetRepo(db),
		repo.NewAttemptRepo(db),
		repo.NewSRSStateRepo(db),
		clock.Fixed{T: now},
	)
	in := domain.SubmitInput{
		QuestionID: qID,
		SetCode:    "core_300",
		Answer: domain.AnswerPayload{
			Type:    domain.QuestionTypeJournal,
			Debits:  []domain.JournalEntry{{Account: "現金", Amount: 1000}},
			Credits: []domain.JournalEntry{{Account: "売上", Amount: 1000}},
		},
		DurationMs: 3000,
	}
	g, err := q.Submit(context.Background(), uid, in)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !g.IsCorrect() {
		t.Fatalf("expected correct")
	}
}

func TestQuizSubmitIncorrectAndDeleteAttempt(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	uid, qID := seedQuiz(t, db)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	q := service.NewQuizService(
		repo.NewQuestionRepo(db), repo.NewSetRepo(db),
		repo.NewAttemptRepo(db), repo.NewSRSStateRepo(db),
		clock.Fixed{T: now},
	)
	in := domain.SubmitInput{
		QuestionID: qID,
		Answer: domain.AnswerPayload{
			Type:    domain.QuestionTypeJournal,
			Debits:  []domain.JournalEntry{{Account: "現金", Amount: 999}},
			Credits: []domain.JournalEntry{{Account: "売上", Amount: 999}},
		},
		DurationMs: 9000,
	}
	g, err := q.Submit(context.Background(), uid, in)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if g.IsCorrect() {
		t.Fatalf("expected incorrect")
	}
	if err := q.DeleteAttempt(context.Background(), uid, g.Attempt.ID); err != nil {
		t.Fatalf("DeleteAttempt: %v", err)
	}
	hist, err := q.History(context.Background(), uid, 10, 0)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 0 {
		t.Fatalf("history len = %d, want 0", len(hist))
	}
}

func TestQuizNextQuestionSequential(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	uid, _ := seedQuiz(t, db)
	q := service.NewQuizService(
		repo.NewQuestionRepo(db), repo.NewSetRepo(db),
		repo.NewAttemptRepo(db), repo.NewSRSStateRepo(db),
		clock.System{},
	)
	got, err := q.NextQuestion(context.Background(), uid, "core_300", domain.QuizModeSequential)
	if err != nil {
		t.Fatalf("NextQuestion: %v", err)
	}
	if got.Code != "q1" {
		t.Fatalf("code = %s", got.Code)
	}
}

// NextQuestion のモード分岐: ランダム/既定 (SRS)/不正モード/空セット。
func TestQuizNextQuestionModeBranches(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	uid, qID := seedQuiz(t, db)
	ctx := context.Background()
	svc := service.NewQuizService(
		repo.NewQuestionRepo(db), repo.NewSetRepo(db),
		repo.NewAttemptRepo(db), repo.NewSRSStateRepo(db),
		clock.System{},
	)

	// ランダムモード: セット唯一の問題が返る。
	got, err := svc.NextQuestion(ctx, uid, "core_300", domain.QuizModeRandom)
	if err != nil || got.ID != qID {
		t.Fatalf("random: got=%v err=%v, want qID=%d", got, err, qID)
	}
	// モード未指定は SRS 扱い。
	got, err = svc.NextQuestion(ctx, uid, "core_300", "")
	if err != nil || got.ID != qID {
		t.Fatalf("default(srs): got=%v err=%v, want qID=%d", got, err, qID)
	}
	// 不正モードは ErrInvalidInput。
	if _, err := svc.NextQuestion(ctx, uid, "core_300", domain.QuizMode("bogus")); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid mode err = %v, want ErrInvalidInput", err)
	}
	// 存在しないセット (メンバー 0 件) は ErrNotFound。
	if _, err := svc.NextQuestion(ctx, uid, "no_such_set", domain.QuizModeRandom); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("empty set err = %v, want ErrNotFound", err)
	}
}

// seedQuizSeqSet は 3 問からなるセット seq_set を持つフィクスチャを作る。
func seedQuizSeqSet(t *testing.T, db *sql.DB) (uid, setID int64, qIDs [3]int64) {
	t.Helper()
	ctx := context.Background()
	res, err := db.ExecContext(ctx,
		`INSERT INTO users(username, password_hash, password_salt, password_params, password_updated_at, created_at, updated_at)
		 VALUES ('sequser', ?, ?, 't', 0, 0, 0)`, []byte("h"), []byte("s"))
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	uid, _ = res.LastInsertId()
	res, err = db.ExecContext(ctx, `INSERT INTO topics(code, name, ord) VALUES ('seq', '順序', 9)`)
	if err != nil {
		t.Fatalf("insert topic: %v", err)
	}
	topicID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx,
		`INSERT INTO question_sets(code, name, description, target_size) VALUES ('seq_set', '順序セット', '', 3)`)
	if err != nil {
		t.Fatalf("insert set: %v", err)
	}
	setID, _ = res.LastInsertId()
	for i := range 3 {
		res, err := db.ExecContext(ctx,
			`INSERT INTO questions(code, topic_id, question_type, difficulty, prompt, payload_json, answer_json, explanation, references_json, created_at)
			 VALUES (?, ?, 'journal', 1, 'p', '{}', '{}', 'e', NULL, 0)`,
			fmt.Sprintf("seq-q%d", i+1), topicID)
		if err != nil {
			t.Fatalf("insert question: %v", err)
		}
		qIDs[i], _ = res.LastInsertId()
		if _, err := db.ExecContext(ctx,
			`INSERT INTO question_set_members(set_id, question_id, ord) VALUES (?, ?, ?)`,
			setID, qIDs[i], i+1); err != nil {
			t.Fatalf("insert member: %v", err)
		}
	}
	return uid, setID, qIDs
}

// 順序通りモードは「そのセットで最後に回答した問題の次」を出題し、末尾の次は先頭に戻る。
func TestQuizNextQuestionSequentialAdvancesAfterEachAnswer(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	uid, setID, qIDs := seedQuizSeqSet(t, db)
	ctx := context.Background()
	svc := service.NewQuizService(
		repo.NewQuestionRepo(db), repo.NewSetRepo(db),
		repo.NewAttemptRepo(db), repo.NewSRSStateRepo(db),
		clock.System{},
	)
	attempts := repo.NewAttemptRepo(db)
	answer := func(qid int64, at time.Time) {
		t.Helper()
		sid := setID
		if err := attempts.Create(ctx, &domain.Attempt{
			UserID: uid, QuestionID: qid, SetID: &sid, IsCorrect: true,
			DurationMs: 1, SubmittedAnswerJSON: `{}`, AnsweredAt: at,
		}); err != nil {
			t.Fatalf("create attempt: %v", err)
		}
	}
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// 未回答 → 先頭。q1 回答 → q2、q2 回答 → q3、q3 回答 → 先頭へ戻る。
	steps := []struct {
		answered int64 // 0 なら回答なし
		want     int64
	}{
		{0, qIDs[0]},
		{qIDs[0], qIDs[1]},
		{qIDs[1], qIDs[2]},
		{qIDs[2], qIDs[0]},
	}
	for i, s := range steps {
		if s.answered != 0 {
			answer(s.answered, base.Add(time.Duration(i)*time.Minute))
		}
		got, err := svc.NextQuestion(ctx, uid, "seq_set", domain.QuizModeSequential)
		if err != nil {
			t.Fatalf("step %d NextQuestion: %v", i, err)
		}
		if got.ID != s.want {
			t.Fatalf("step %d: got qID=%d, want %d", i, got.ID, s.want)
		}
	}
}

// 最後に回答した問題がセットから外れていた場合は先頭に戻る。
func TestQuizNextQuestionSequentialFallsBackToFirstWhenLastRemoved(t *testing.T) {
	t.Parallel()
	db := newTestServiceDB(t)
	uid, setID, qIDs := seedQuizSeqSet(t, db)
	ctx := context.Background()
	// セット外の問題 (メンバー登録なし) への回答を set_id 付きで残す。
	res, err := db.ExecContext(ctx,
		`INSERT INTO questions(code, topic_id, question_type, difficulty, prompt, payload_json, answer_json, explanation, references_json, created_at)
		 VALUES ('seq-gone', (SELECT topic_id FROM questions WHERE id = ?), 'journal', 1, 'p', '{}', '{}', 'e', NULL, 0)`,
		qIDs[0])
	if err != nil {
		t.Fatalf("insert outside question: %v", err)
	}
	goneID, _ := res.LastInsertId()
	sid := setID
	if err := repo.NewAttemptRepo(db).Create(ctx, &domain.Attempt{
		UserID: uid, QuestionID: goneID, SetID: &sid, IsCorrect: true,
		DurationMs: 1, SubmittedAnswerJSON: `{}`,
	}); err != nil {
		t.Fatalf("create attempt: %v", err)
	}
	svc := service.NewQuizService(
		repo.NewQuestionRepo(db), repo.NewSetRepo(db),
		repo.NewAttemptRepo(db), repo.NewSRSStateRepo(db),
		clock.System{},
	)
	got, err := svc.NextQuestion(ctx, uid, "seq_set", domain.QuizModeSequential)
	if err != nil {
		t.Fatalf("NextQuestion: %v", err)
	}
	if got.ID != qIDs[0] {
		t.Fatalf("got qID=%d, want %d (先頭へフォールバック)", got.ID, qIDs[0])
	}
}
