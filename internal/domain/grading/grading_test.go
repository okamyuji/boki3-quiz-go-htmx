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

// 勘定科目の表記ゆれ (送り仮名・空白混入) は標準科目名へ正規化して採点する。
func TestJournalAccountVariantsGradeAsCorrect(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		wantDebit  string
		gotDebit   string
		wantCredit string
		gotCredit  string
	}{
		{"売り上げ→売上", "現金", "現金", "売上", "売り上げ"},
		{"売上げ→売上", "現金", "現金", "売上", "売上げ"},
		{"仕入れ→仕入", "仕入", "仕入れ", "買掛金", "買掛金"},
		{"半角空白の混入", "現金", "現 金", "売上", "売上"},
		{"全角空白の混入", "現金", "現　金", "売上", "売上"},
		{"繰り越し商品→繰越商品", "仕入", "仕入", "繰越商品", "繰り越し商品"},
		{"前払い金→前払金", "前払金", "前払い金", "現金", "現金"},
		{"未収金→未収入金", "未収入金", "未収金", "土地", "土地"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			want := domain.AnswerPayload{
				Type:    domain.QuestionTypeJournal,
				Debits:  []domain.JournalEntry{{Account: c.wantDebit, Amount: 5000}},
				Credits: []domain.JournalEntry{{Account: c.wantCredit, Amount: 5000}},
			}
			got := domain.AnswerPayload{
				Type:    domain.QuestionTypeJournal,
				Debits:  []domain.JournalEntry{{Account: c.gotDebit, Amount: 5000}},
				Credits: []domain.JournalEntry{{Account: c.gotCredit, Amount: 5000}},
			}
			if !grading.IsCorrect(want, got) {
				t.Fatalf("%s: variant must grade as correct", c.name)
			}
		})
	}
}

// 正規化は別科目を同一視しない: 1 字違いの科目・未知の表記は不正解のまま。
func TestJournalDifferentAccountsStayIncorrect(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		got  string
	}{
		{"売掛金と買掛金は別科目", "買掛金"},
		{"かな書きは救済しない", "うりあげ"},
		{"未知の科目名", "売上高"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			want := domain.AnswerPayload{
				Type:    domain.QuestionTypeJournal,
				Debits:  []domain.JournalEntry{{Account: "現金", Amount: 5000}},
				Credits: []domain.JournalEntry{{Account: "売上", Amount: 5000}},
			}
			got := domain.AnswerPayload{
				Type:    domain.QuestionTypeJournal,
				Debits:  []domain.JournalEntry{{Account: "現金", Amount: 5000}},
				Credits: []domain.JournalEntry{{Account: c.got, Amount: 5000}},
			}
			if grading.IsCorrect(want, got) {
				t.Fatalf("%s: must stay incorrect", c.name)
			}
		})
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
