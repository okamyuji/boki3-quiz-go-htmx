package seed_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/accounts"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/grading"
	"github.com/okamyuji/boki3-quiz-go-htmx/seed"
)

// 質重視バンクの総数要件: 300 前後 (280〜330)。
func TestGenerateTotalInQualityRange(t *testing.T) {
	t.Parallel()
	n := len(seed.Generate())
	if n < 280 || n > 330 {
		t.Fatalf("generated %d, want 280..330 (質重視 300 前後)", n)
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

// 完全重複問題の禁止: prompt は全問でユニーク。
func TestGeneratePromptsUnique(t *testing.T) {
	t.Parallel()
	seen := map[string]string{}
	for _, q := range seed.Generate() {
		if prev, ok := seen[q.Prompt]; ok {
			t.Fatalf("duplicate prompt: %s and %s share %q", prev, q.Code, q.Prompt)
		}
		seen[q.Prompt] = q.Code
	}
}

var variantSuffix = regexp.MustCompile(`-\d{3}$`)

// patternOf は code から金額バリアント連番を除いた文型キーを返す。
func patternOf(code string) string {
	return variantSuffix.ReplaceAllString(code, "")
}

// 文型の多様性: 全体で 100 文型以上、各論点 5 文型以上。
func TestGeneratePatternDiversity(t *testing.T) {
	t.Parallel()
	patterns := map[string]bool{}
	perTopic := map[string]map[string]bool{}
	for _, q := range seed.Generate() {
		p := patternOf(q.Code)
		patterns[p] = true
		if perTopic[q.TopicCode] == nil {
			perTopic[q.TopicCode] = map[string]bool{}
		}
		perTopic[q.TopicCode][p] = true
	}
	if len(patterns) < 100 {
		t.Fatalf("distinct patterns = %d, want >= 100", len(patterns))
	}
	for topic, ps := range perTopic {
		if len(ps) < 5 {
			t.Fatalf("topic %s has %d patterns, want >= 5", topic, len(ps))
		}
	}
}

// 金額バリアントは 1 文型あたり最大 3 問。
func TestGenerateVariantsAtMostThree(t *testing.T) {
	t.Parallel()
	counts := map[string]int{}
	for _, q := range seed.Generate() {
		counts[patternOf(q.Code)]++
	}
	for p, c := range counts {
		if c > 3 {
			t.Fatalf("pattern %s has %d variants, want <= 3", p, c)
		}
	}
}

// 3 級の標準勘定科目 (accounts.Standard) のみを使用する (誤字・級外科目の混入防止)。
func TestGenerateJournalAccountsWhitelisted(t *testing.T) {
	t.Parallel()
	allowed := map[string]bool{}
	for _, a := range accounts.Standard {
		allowed[a] = true
	}
	for _, q := range seed.Generate() {
		for _, e := range append(append([]domain.JournalEntry{}, q.Answer.Debits...), q.Answer.Credits...) {
			if !allowed[e.Account] {
				t.Fatalf("question %s uses non-whitelisted account %q", q.Code, e.Account)
			}
			if e.Amount <= 0 {
				t.Fatalf("question %s has non-positive amount %d for %s", q.Code, e.Amount, e.Account)
			}
		}
	}
}

// 現行 3 級 (小規模株式会社前提) の範囲外である個人商店の語彙を禁止する。
func TestGenerateNoOutOfScopeVocabulary(t *testing.T) {
	t.Parallel()
	for _, q := range seed.Generate() {
		for _, banned := range []string{"元入れ", "店主", "引出金"} {
			if strings.Contains(q.Prompt, banned) || strings.Contains(q.Explanation, banned) {
				t.Fatalf("question %s contains out-of-scope term %q", q.Code, banned)
			}
		}
	}
}

func TestGenerateAnswersGradeAsCorrect(t *testing.T) {
	t.Parallel()
	for _, q := range seed.Generate() {
		// 同じ Answer を submit すれば必ず正解になるはず
		if !grading.IsCorrect(q.Answer, q.Answer) {
			t.Fatalf("question %s: self-grading not correct (likely answer/type mismatch)", q.Code)
		}
		if q.Answer.Type == domain.QuestionTypeJournal {
			if len(q.Answer.Debits) == 0 || len(q.Answer.Credits) == 0 {
				t.Fatalf("question %s: journal must have both sides", q.Code)
			}
			if sum(q.Answer.Debits) != sum(q.Answer.Credits) {
				t.Fatalf("question %s: debits != credits", q.Code)
			}
		}
	}
}

// セット構成規則:
//   - 仕訳問題は journal_240 と comprehensive_300 に属し、第 1 バリアントは core_300 にも属する。
//   - 選択式は core_300 と comprehensive_300 に属する。
func TestGenerateSetComposition(t *testing.T) {
	t.Parallel()
	for _, q := range seed.Generate() {
		in := map[string]bool{}
		for _, s := range q.Sets {
			in[s] = true
		}
		if !in[domain.SetCodeComprehensive] {
			t.Fatalf("question %s missing comprehensive set", q.Code)
		}
		if q.Answer.Type == domain.QuestionTypeJournal {
			if !in[domain.SetCodeJournal] {
				t.Fatalf("journal question %s missing journal set", q.Code)
			}
			isFirst := strings.HasSuffix(q.Code, "-001")
			if isFirst != in[domain.SetCodeCore] {
				t.Fatalf("question %s: core membership = %v, want %v (第1バリアントのみ core)", q.Code, in[domain.SetCodeCore], isFirst)
			}
		} else if !in[domain.SetCodeCore] {
			t.Fatalf("choice question %s missing core set", q.Code)
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

// 選択式問題は choices を持ち、解答が選択肢に含まれる。
func TestGenerateChoiceQuestionsWellFormed(t *testing.T) {
	t.Parallel()
	for _, q := range seed.Generate() {
		if q.Answer.Type == domain.QuestionTypeJournal {
			continue
		}
		raw, ok := q.Payload["choices"]
		if !ok {
			t.Fatalf("choice question %s has no choices payload", q.Code)
		}
		choices, ok := raw.([]string)
		if !ok || len(choices) < 3 {
			t.Fatalf("choice question %s: choices must be >=3 strings, got %#v", q.Code, raw)
		}
		found := false
		for _, c := range choices {
			if strings.HasPrefix(c, q.Answer.Choice+":") {
				found = true
			}
		}
		if !found {
			t.Fatalf("choice question %s: answer %q not among choices %v", q.Code, q.Answer.Choice, choices)
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
