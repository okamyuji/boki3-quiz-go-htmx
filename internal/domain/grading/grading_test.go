package grading_test

import (
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/grading"
)

func TestJournalCorrectAnyOrder(t *testing.T) {
	t.Parallel()
	want := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "現金", Amount: 10000}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 10000}},
	}
	got := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "現金", Amount: 10000}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 10000}},
	}
	if !grading.IsCorrect(want, got) {
		t.Fatalf("expected correct match")
	}
}

func TestJournalReorderedStillCorrect(t *testing.T) {
	t.Parallel()
	want := domain.AnswerPayload{
		Type: domain.QuestionTypeJournal,
		Debits: []domain.JournalEntry{
			{Account: "現金", Amount: 7000},
			{Account: "売掛金", Amount: 3000},
		},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 10000}},
	}
	got := domain.AnswerPayload{
		Type: domain.QuestionTypeJournal,
		Debits: []domain.JournalEntry{
			{Account: "売掛金", Amount: 3000},
			{Account: "現金", Amount: 7000},
		},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 10000}},
	}
	if !grading.IsCorrect(want, got) {
		t.Fatalf("expected correct (order shuffled)")
	}
}

func TestJournalEmptyRowsIgnored(t *testing.T) {
	t.Parallel()
	want := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "現金", Amount: 5000}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 5000}},
	}
	got := domain.AnswerPayload{
		Type: domain.QuestionTypeJournal,
		Debits: []domain.JournalEntry{
			{Account: "現金", Amount: 5000},
			{Account: "", Amount: 0},
		},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 5000}},
	}
	if !grading.IsCorrect(want, got) {
		t.Fatalf("expected correct (empty rows ignored)")
	}
}

func TestJournalAmountMismatchIncorrect(t *testing.T) {
	t.Parallel()
	want := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "現金", Amount: 5000}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 5000}},
	}
	got := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "現金", Amount: 4999}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 4999}},
	}
	if grading.IsCorrect(want, got) {
		t.Fatalf("expected incorrect (amount mismatch)")
	}
}

func TestJournalAccountMismatchIncorrect(t *testing.T) {
	t.Parallel()
	want := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "現金", Amount: 5000}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 5000}},
	}
	got := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "当座預金", Amount: 5000}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 5000}},
	}
	if grading.IsCorrect(want, got) {
		t.Fatalf("expected incorrect (account mismatch)")
	}
}

func TestChoiceMatch(t *testing.T) {
	t.Parallel()
	want := domain.AnswerPayload{Type: domain.QuestionTypeSubbook, Choice: "A"}
	got := domain.AnswerPayload{Type: domain.QuestionTypeSubbook, Choice: "a"}
	if !grading.IsCorrect(want, got) {
		t.Fatalf("expected case-insensitive choice match")
	}
}

func TestUnknownTypeIncorrect(t *testing.T) {
	t.Parallel()
	want := domain.AnswerPayload{Type: domain.QuestionType("bogus")}
	got := domain.AnswerPayload{Type: domain.QuestionType("bogus")}
	if grading.IsCorrect(want, got) {
		t.Fatalf("expected incorrect for unknown type")
	}
}

func TestTypeMismatch(t *testing.T) {
	t.Parallel()
	want := domain.AnswerPayload{Type: domain.QuestionTypeJournal}
	got := domain.AnswerPayload{Type: domain.QuestionTypeSubbook}
	if grading.IsCorrect(want, got) {
		t.Fatalf("expected incorrect when types differ")
	}
}
