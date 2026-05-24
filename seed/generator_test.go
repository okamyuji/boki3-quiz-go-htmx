package seed_test

import (
	"strings"
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/grading"
	"github.com/okamyuji/boki3-quiz-go-htmx/seed"
)

func TestGenerateProducesAtLeast430(t *testing.T) {
	t.Parallel()
	qs := seed.Generate()
	if len(qs) < 430 {
		t.Fatalf("generated %d, want >= 430", len(qs))
	}
}

func TestGenerateCodesUnique(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, q := range seed.Generate() {
		if seen[q.Code] {
			t.Fatalf("duplicate code %q", q.Code)
		}
		seen[q.Code] = true
	}
}

func TestGenerateAnswersGradeAsCorrect(t *testing.T) {
	t.Parallel()
	for _, q := range seed.Generate() {
		// 同じ Answer を submit すれば必ず正解になるはず
		if !grading.IsCorrect(q.Answer, q.Answer) {
			t.Fatalf("question %s: self-grading not correct (likely answer/type mismatch)", q.Code)
		}
		switch q.Answer.Type {
		case domain.QuestionTypeJournal:
			if len(q.Answer.Debits) == 0 || len(q.Answer.Credits) == 0 {
				t.Fatalf("question %s: journal must have both sides", q.Code)
			}
			if sum(q.Answer.Debits) != sum(q.Answer.Credits) {
				t.Fatalf("question %s: debits != credits", q.Code)
			}
		}
	}
}

func TestGenerateSetsReferToKnownCodes(t *testing.T) {
	t.Parallel()
	valid := map[string]bool{
		domain.SetCodeCore:          true,
		domain.SetCodeJournal:       true,
		domain.SetCodeComprehensive: true,
	}
	for _, q := range seed.Generate() {
		for _, s := range q.Sets {
			if !valid[s] {
				t.Fatalf("question %s references unknown set %q", q.Code, s)
			}
		}
		if len(q.Sets) == 0 {
			t.Fatalf("question %s belongs to no set", q.Code)
		}
	}
}

func TestPromptsAreNonEmpty(t *testing.T) {
	t.Parallel()
	for _, q := range seed.Generate() {
		if strings.TrimSpace(q.Prompt) == "" {
			t.Fatalf("question %s has empty prompt", q.Code)
		}
		if strings.TrimSpace(q.Explanation) == "" {
			t.Fatalf("question %s has empty explanation", q.Code)
		}
	}
}

func sum(es []domain.JournalEntry) int64 {
	var s int64
	for _, e := range es {
		s += e.Amount
	}
	return s
}
