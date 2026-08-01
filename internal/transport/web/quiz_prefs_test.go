package web

import (
	"slices"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

func setExistsIn(codes ...string) func(string) bool {
	return func(c string) bool { return slices.Contains(codes, c) }
}

func TestResolveQuizPrefsDefaultsWhenNoQueryAndNoStored(t *testing.T) {
	t.Parallel()
	set, mode, save := resolveQuizPrefs(quizPrefsInput{
		SetExists: setExistsIn(domain.SetCodeCore),
	})
	if set != domain.SetCodeCore {
		t.Fatalf("set = %q, want %q", set, domain.SetCodeCore)
	}
	if mode != domain.QuizModeSRS {
		t.Fatalf("mode = %q, want %q", mode, domain.QuizModeSRS)
	}
	if save {
		t.Fatal("save = true, want false (nothing to persist)")
	}
}

func TestResolveQuizPrefsRestoresStoredWhenNoQuery(t *testing.T) {
	t.Parallel()
	stored := &domain.UserPrefs{QuizSet: "journal_150", QuizMode: domain.QuizModeRandom}
	set, mode, save := resolveQuizPrefs(quizPrefsInput{
		Stored:    stored,
		SetExists: setExistsIn(domain.SetCodeCore, "journal_150"),
	})
	if set != "journal_150" {
		t.Fatalf("set = %q, want stored journal_150", set)
	}
	if mode != domain.QuizModeRandom {
		t.Fatalf("mode = %q, want stored random", mode)
	}
	if save {
		t.Fatal("save = true, want false (no explicit switch)")
	}
}

func TestResolveQuizPrefsQueryWinsAndRequestsSave(t *testing.T) {
	t.Parallel()
	stored := &domain.UserPrefs{QuizSet: domain.SetCodeCore, QuizMode: domain.QuizModeSRS}
	set, mode, save := resolveQuizPrefs(quizPrefsInput{
		QuerySet:  "journal_150",
		QueryMode: "sequential",
		Stored:    stored,
		SetExists: setExistsIn(domain.SetCodeCore, "journal_150"),
	})
	if set != "journal_150" || mode != domain.QuizModeSequential {
		t.Fatalf("(set, mode) = (%q, %q), want query values", set, mode)
	}
	if !save {
		t.Fatal("save = false, want true (explicit switch differs from stored)")
	}
}

func TestResolveQuizPrefsSaveOnFirstExplicitChoiceWithoutStored(t *testing.T) {
	t.Parallel()
	set, mode, save := resolveQuizPrefs(quizPrefsInput{
		QuerySet:  domain.SetCodeCore,
		QueryMode: "srs",
		SetExists: setExistsIn(domain.SetCodeCore),
	})
	if set != domain.SetCodeCore || mode != domain.QuizModeSRS {
		t.Fatalf("(set, mode) = (%q, %q), want defaults", set, mode)
	}
	if !save {
		t.Fatal("save = false, want true (first explicit choice must persist)")
	}
}

func TestResolveQuizPrefsNoSaveWhenQueryEqualsStored(t *testing.T) {
	t.Parallel()
	stored := &domain.UserPrefs{QuizSet: "journal_150", QuizMode: domain.QuizModeRandom}
	_, _, save := resolveQuizPrefs(quizPrefsInput{
		QuerySet:  "journal_150",
		QueryMode: "random",
		Stored:    stored,
		SetExists: setExistsIn("journal_150"),
	})
	if save {
		t.Fatal("save = true, want false (query equals stored)")
	}
}

func TestResolveQuizPrefsPartialQueryFillsRestFromStored(t *testing.T) {
	t.Parallel()
	stored := &domain.UserPrefs{QuizSet: domain.SetCodeCore, QuizMode: domain.QuizModeRandom}
	set, mode, save := resolveQuizPrefs(quizPrefsInput{
		QuerySet:  "journal_150",
		Stored:    stored,
		SetExists: setExistsIn(domain.SetCodeCore, "journal_150"),
	})
	if set != "journal_150" {
		t.Fatalf("set = %q, want query journal_150", set)
	}
	if mode != domain.QuizModeRandom {
		t.Fatalf("mode = %q, want stored random", mode)
	}
	if !save {
		t.Fatal("save = false, want true (set switched)")
	}
}

func TestResolveQuizPrefsInvalidQueryFallsBackToStored(t *testing.T) {
	t.Parallel()
	stored := &domain.UserPrefs{QuizSet: "journal_150", QuizMode: domain.QuizModeRandom}
	set, mode, save := resolveQuizPrefs(quizPrefsInput{
		QuerySet:  "no_such_set",
		QueryMode: "bogus",
		Stored:    stored,
		SetExists: setExistsIn(domain.SetCodeCore, "journal_150"),
	})
	if set != "journal_150" || mode != domain.QuizModeRandom {
		t.Fatalf("(set, mode) = (%q, %q), want stored values", set, mode)
	}
	if save {
		t.Fatal("save = true, want false (invalid query must not persist)")
	}
}

func TestResolveQuizPrefsInvalidStoredSetFallsBackToDefault(t *testing.T) {
	t.Parallel()
	stored := &domain.UserPrefs{QuizSet: "deleted_set", QuizMode: "bogus", UpdatedAt: time.Unix(0, 0)}
	set, mode, save := resolveQuizPrefs(quizPrefsInput{
		Stored:    stored,
		SetExists: setExistsIn(domain.SetCodeCore),
	})
	if set != domain.SetCodeCore {
		t.Fatalf("set = %q, want default (stored set no longer exists)", set)
	}
	if mode != domain.QuizModeSRS {
		t.Fatalf("mode = %q, want default (stored mode invalid)", mode)
	}
	if save {
		t.Fatal("save = true, want false")
	}
}
