// Package seed は問題バンクの生成と DB 同期を担う。
//
// 問題バンクは「実文型 (pattern) × 金額バリアント (最大 3)」で構成する。
// 文型は日商簿記 3 級 (2022 年度出題区分表、小規模株式会社前提) の取引を
// 手動で設計・会計検証したものに限定し、派生額 (税額・利息・簿価・準備金等)
// は整数で割り切れる比率のみを用いる。数字違いの量産で件数を稼がない
// (バリアントは 1 文型あたり最大 3 問)。
package seed

import (
	"encoding/json"
	"fmt"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// Question は seed JSON 用の DTO (domain.Question より広めの公開項目)。
type Question struct {
	Code         string               `json:"code"`
	TopicCode    string               `json:"topic_code"`
	QuestionType string               `json:"question_type"`
	Difficulty   int                  `json:"difficulty"`
	Prompt       string               `json:"prompt"`
	Payload      map[string]any       `json:"payload"`
	Answer       domain.AnswerPayload `json:"answer"`
	Explanation  string               `json:"explanation"`
	References   []string             `json:"references"`
	Sets         []string             `json:"sets"` // 所属するセット code
}

// pattern は 1 実文型。バリアント番号 i (0 始まり) ごとに 1 問を生成する。
type pattern struct {
	prefix string // code 接頭辞 (バンク内で一意)
	topic  string
	diff   int
	n      int // 金額バリアント数 (1..3)
	expl   string
	build  func(i int) (prompt string, debits, credits []domain.JournalEntry)
}

// Generate は問題バンク全体を返す。
func Generate() []Question {
	out := make([]Question, 0, 320)
	out = append(out, expandPatterns(merchPatterns())...)
	out = append(out, expandPatterns(cashPatterns())...)
	out = append(out, expandPatterns(receivablePatterns())...)
	out = append(out, expandPatterns(notesLoanPatterns())...)
	out = append(out, expandPatterns(fixedOtherPatterns())...)
	out = append(out, expandPatterns(taxWageCapitalPatterns())...)
	out = append(out, expandPatterns(adjustmentPatterns())...)
	out = append(out, choiceQuestions()...)
	return out
}

// ToJSON はシード全体を JSON にシリアライズする。
func ToJSON() ([]byte, error) {
	return json.MarshalIndent(Generate(), "", "  ")
}

// expandPatterns は文型をバリアント展開する。
// セット構成規則: 仕訳問題は journal/comprehensive に属し、第 1 バリアントのみ core にも属する。
func expandPatterns(ps []pattern) []Question {
	out := make([]Question, 0, len(ps)*3)
	for _, p := range ps {
		for i := range p.n {
			prompt, debits, credits := p.build(i)
			sets := []string{domain.SetCodeJournal, domain.SetCodeComprehensive}
			if i == 0 {
				sets = append(sets, domain.SetCodeCore)
			}
			q := journalQuestion(fmt.Sprintf("%s-%03d", p.prefix, i+1), p.topic, prompt, p.expl, debits, credits, sets...)
			q.Difficulty = p.diff
			out = append(out, q)
		}
	}
	return out
}

// pick は i 番目のバリアント値を返す。
func pick(i int, vals ...int64) int64 { return vals[i] }

// je は仕訳エントリを作る。
func je(account string, amount int64) domain.JournalEntry {
	return domain.JournalEntry{Account: account, Amount: amount}
}

// side は仕訳の片側 (借方または貸方) を組み立てる。
func side(entries ...domain.JournalEntry) []domain.JournalEntry { return entries }

func journalQuestion(code, topic, prompt, explanation string,
	debits, credits []domain.JournalEntry, sets ...string,
) Question {
	return Question{
		Code:         code,
		TopicCode:    topic,
		QuestionType: string(domain.QuestionTypeJournal),
		Difficulty:   1,
		Prompt:       prompt,
		Payload:      map[string]any{"hint": "借方/貸方の勘定科目と金額を入力してください。"},
		Answer: domain.AnswerPayload{
			Type:    domain.QuestionTypeJournal,
			Debits:  debits,
			Credits: credits,
		},
		Explanation: explanation,
		References:  []string{"日商簿記3級 公式出題範囲 (kentei.ne.jp)"},
		Sets:        sets,
	}
}

// choiceQuestion は選択式問題を作る。core と comprehensive に属する。
func choiceQuestion(code, topic string, qt domain.QuestionType, diff int,
	prompt string, choices []string, answer, explanation string,
) Question {
	return Question{
		Code:         code,
		TopicCode:    topic,
		QuestionType: string(qt),
		Difficulty:   diff,
		Prompt:       prompt,
		Payload:      map[string]any{"choices": choices},
		Answer:       domain.AnswerPayload{Type: qt, Choice: answer},
		Explanation:  explanation,
		References:   []string{"日商簿記3級 公式出題範囲 (kentei.ne.jp)"},
		Sets:         []string{domain.SetCodeCore, domain.SetCodeComprehensive},
	}
}

func formatYen(n int64) string {
	// 1,000,000 形式の桁区切り
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s + " 円"
	}
	// 簡易 3 桁区切り
	var rev []byte
	for i := len(s) - 1; i >= 0; i-- {
		rev = append(rev, s[i])
	}
	var out []byte
	for i, c := range rev {
		if i > 0 && i%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	// reverse
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out) + " 円"
}
