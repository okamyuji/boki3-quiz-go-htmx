// Package grading は提出解答の正誤判定を提供する純関数集。
package grading

import (
	"sort"
	"strings"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// IsCorrect は want (正答) と got (提出) が一致するか判定する。
// 仕訳は (科目, 金額) の多重集合一致、選択肢は大文字小文字を無視した一致、テキストはトリム後の一致。
func IsCorrect(want, got domain.AnswerPayload) bool {
	if want.Type != got.Type {
		return false
	}
	switch want.Type {
	case domain.QuestionTypeJournal, domain.QuestionTypeLedger, domain.QuestionTypeSlip:
		return entrySetEqual(want.Debits, got.Debits) && entrySetEqual(want.Credits, got.Credits)
	case domain.QuestionTypeSubbook, domain.QuestionTypeTrialBalance,
		domain.QuestionTypeWorksheet, domain.QuestionTypeFS:
		if want.Choice != "" || got.Choice != "" {
			return strings.EqualFold(strings.TrimSpace(want.Choice), strings.TrimSpace(got.Choice))
		}
		return strings.TrimSpace(want.Text) == strings.TrimSpace(got.Text)
	default:
		return false
	}
}

func entrySetEqual(a, b []domain.JournalEntry) bool {
	na := normalize(a)
	nb := normalize(b)
	if len(na) != len(nb) {
		return false
	}
	for i := range na {
		if na[i] != nb[i] {
			return false
		}
	}
	return true
}

// normalize は空行を除き、(科目, 金額) を辞書順にソートしたコピーを返す。
func normalize(entries []domain.JournalEntry) []domain.JournalEntry {
	out := make([]domain.JournalEntry, 0, len(entries))
	for _, e := range entries {
		acct := strings.TrimSpace(e.Account)
		if acct == "" && e.Amount == 0 {
			continue
		}
		out = append(out, domain.JournalEntry{Account: acct, Amount: e.Amount})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Account != out[j].Account {
			return out[i].Account < out[j].Account
		}
		return out[i].Amount < out[j].Amount
	})
	return out
}
