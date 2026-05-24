package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
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
