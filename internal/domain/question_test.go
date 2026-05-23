package domain_test

import (
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

func TestQuestionTypeValid(t *testing.T) {
	t.Parallel()
	valid := []domain.QuestionType{
		domain.QuestionTypeJournal,
		domain.QuestionTypeLedger,
		domain.QuestionTypeSubbook,
		domain.QuestionTypeTrialBalance,
		domain.QuestionTypeWorksheet,
		domain.QuestionTypeFS,
		domain.QuestionTypeSlip,
	}
	for _, qt := range valid {
		t.Run(string(qt), func(t *testing.T) {
			t.Parallel()
			if !qt.IsValid() {
				t.Fatalf("%q.IsValid() = false", qt)
			}
		})
	}
	if domain.QuestionType("bogus").IsValid() {
		t.Fatalf("bogus.IsValid() = true")
	}
}

func TestQuizModeValid(t *testing.T) {
	t.Parallel()
	for _, m := range []domain.QuizMode{
		domain.QuizModeSRS, domain.QuizModeSequential, domain.QuizModeRandom,
	} {
		if !m.IsValid() {
			t.Fatalf("%q.IsValid() = false", m)
		}
	}
	if domain.QuizMode("nope").IsValid() {
		t.Fatalf("nope.IsValid() = true")
	}
}
