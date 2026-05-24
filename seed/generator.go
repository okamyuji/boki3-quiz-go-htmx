// Package seed は問題シードの生成と JSON 出力を担う。
//
// 本パッケージは「テンプレート + パラメータバリアント」で日商簿記 3 級の代表的な仕訳問題を
// 数百件レンジで生成する。すべての問題は手動レビュー済みの会計的に正しいパターンに限定する。
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

// Generate は問題セット全体を返す。
func Generate() []Question {
	out := make([]Question, 0, 500)
	out = append(out, cashSales(50)...)
	out = append(out, cashPurchases(50)...)
	out = append(out, bankTransfers(30)...)
	out = append(out, creditSales(40)...)
	out = append(out, creditPurchases(40)...)
	out = append(out, notesReceivable(20)...)
	out = append(out, notesPayable(20)...)
	out = append(out, inventoryClose(30)...)
	out = append(out, fixedAssets(30)...)
	out = append(out, loansAndBorrow(20)...)
	out = append(out, otherRecv(20)...)
	out = append(out, taxQuestions(20)...)
	out = append(out, wagesQuestions(15)...)
	out = append(out, capitalQuestions(15)...)
	out = append(out, adjustmentQuestions(40)...)
	out = append(out, trialBalanceQuestions(10)...)
	out = append(out, worksheetQuestions(10)...)
	out = append(out, slipQuestions(10)...)
	return out
}

// ToJSON はシード全体を JSON にシリアライズする。
func ToJSON() ([]byte, error) {
	return json.MarshalIndent(Generate(), "", "  ")
}

// helper: amount sequence 1000, 2000, ..., n*1000
func amounts(n int) []int64 {
	out := make([]int64, n)
	for i := range n {
		out[i] = int64((i + 1) * 1000)
	}
	return out
}

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

func cashSales(n int) []Question {
	out := make([]Question, 0, n)
	for i, amt := range amounts(n) {
		amtStr := formatYen(amt)
		q := journalQuestion(
			fmt.Sprintf("cash-sale-%03d", i+1),
			"cash",
			fmt.Sprintf("商品 %s を売り上げ、代金を現金で受け取った。", amtStr),
			"現金売上の取引である。借方は資産の増加 (現金)、貸方は収益の発生 (売上) で処理する。",
			[]domain.JournalEntry{{Account: "現金", Amount: amt}},
			[]domain.JournalEntry{{Account: "売上", Amount: amt}},
			"core_300", "journal_240",
		)
		out = append(out, q)
	}
	return out
}

func cashPurchases(n int) []Question {
	out := make([]Question, 0, n)
	for i, amt := range amounts(n) {
		amtStr := formatYen(amt)
		q := journalQuestion(
			fmt.Sprintf("cash-purch-%03d", i+1),
			"cash",
			fmt.Sprintf("商品 %s を仕入れ、代金を現金で支払った。", amtStr),
			"現金仕入の取引である。借方は費用 (仕入) の発生、貸方は資産 (現金) の減少。",
			[]domain.JournalEntry{{Account: "仕入", Amount: amt}},
			[]domain.JournalEntry{{Account: "現金", Amount: amt}},
			"core_300", "journal_240",
		)
		out = append(out, q)
	}
	return out
}

func bankTransfers(n int) []Question {
	out := make([]Question, 0, n)
	for i, amt := range amounts(n) {
		amtStr := formatYen(amt)
		q := journalQuestion(
			fmt.Sprintf("bank-dep-%03d", i+1),
			"cash",
			fmt.Sprintf("現金 %s を当座預金に預け入れた。", amtStr),
			"現金から当座預金への振替。借方は当座預金の増加、貸方は現金の減少。",
			[]domain.JournalEntry{{Account: "当座預金", Amount: amt}},
			[]domain.JournalEntry{{Account: "現金", Amount: amt}},
			"core_300", "journal_240",
		)
		out = append(out, q)
	}
	return out
}

func creditSales(n int) []Question {
	out := make([]Question, 0, n)
	for i, amt := range amounts(n) {
		amtStr := formatYen(amt)
		q := journalQuestion(
			fmt.Sprintf("credit-sale-%03d", i+1),
			"receivable",
			fmt.Sprintf("商品 %s を掛けで売り上げた。", amtStr),
			"掛売上では、借方に売掛金 (将来の入金権利)、貸方に売上を計上する。",
			[]domain.JournalEntry{{Account: "売掛金", Amount: amt}},
			[]domain.JournalEntry{{Account: "売上", Amount: amt}},
			"core_300", "journal_240", "comprehensive_300",
		)
		out = append(out, q)
	}
	return out
}

func creditPurchases(n int) []Question {
	out := make([]Question, 0, n)
	for i, amt := range amounts(n) {
		amtStr := formatYen(amt)
		q := journalQuestion(
			fmt.Sprintf("credit-purch-%03d", i+1),
			"receivable",
			fmt.Sprintf("商品 %s を掛けで仕入れた。", amtStr),
			"掛仕入では、借方に仕入、貸方に買掛金を計上する。",
			[]domain.JournalEntry{{Account: "仕入", Amount: amt}},
			[]domain.JournalEntry{{Account: "買掛金", Amount: amt}},
			"core_300", "journal_240", "comprehensive_300",
		)
		out = append(out, q)
	}
	return out
}

func notesReceivable(n int) []Question {
	out := make([]Question, 0, n)
	for i, amt := range amounts(n) {
		amtStr := formatYen(amt)
		q := journalQuestion(
			fmt.Sprintf("notes-recv-%03d", i+1),
			"notes",
			fmt.Sprintf("商品 %s を売り上げ、代金は約束手形で受け取った。", amtStr),
			"手形での売上は、借方に受取手形、貸方に売上を計上する。",
			[]domain.JournalEntry{{Account: "受取手形", Amount: amt}},
			[]domain.JournalEntry{{Account: "売上", Amount: amt}},
			"core_300", "journal_240",
		)
		out = append(out, q)
	}
	return out
}

func notesPayable(n int) []Question {
	out := make([]Question, 0, n)
	for i, amt := range amounts(n) {
		amtStr := formatYen(amt)
		q := journalQuestion(
			fmt.Sprintf("notes-pay-%03d", i+1),
			"notes",
			fmt.Sprintf("商品 %s を仕入れ、代金は約束手形を振り出した。", amtStr),
			"手形での仕入は、借方に仕入、貸方に支払手形を計上する。",
			[]domain.JournalEntry{{Account: "仕入", Amount: amt}},
			[]domain.JournalEntry{{Account: "支払手形", Amount: amt}},
			"core_300", "journal_240",
		)
		out = append(out, q)
	}
	return out
}

func inventoryClose(n int) []Question {
	out := make([]Question, 0, n)
	for i := range n {
		beg := int64((i + 1) * 10000)
		end := int64((i + 1) * 8000)
		q := journalQuestion(
			fmt.Sprintf("inv-close-%03d", i+1),
			"inventory",
			fmt.Sprintf("期末整理: 期首商品棚卸高 %s、期末商品棚卸高 %s を仕入勘定で振替える (三分法)。",
				formatYen(beg), formatYen(end)),
			"三分法での売上原価算定: 仕入 / 繰越商品 (期首) で繰越商品を減らし、繰越商品 / 仕入 (期末) で在庫戻し。",
			[]domain.JournalEntry{
				{Account: "仕入", Amount: beg},
				{Account: "繰越商品", Amount: end},
			},
			[]domain.JournalEntry{
				{Account: "繰越商品", Amount: beg},
				{Account: "仕入", Amount: end},
			},
			"core_300", "comprehensive_300",
		)
		q.QuestionType = string(domain.QuestionTypeJournal)
		q.Difficulty = 2
		out = append(out, q)
	}
	return out
}

func fixedAssets(n int) []Question {
	out := make([]Question, 0, n)
	for i, amt := range amounts(n) {
		amtStr := formatYen(amt * 10)
		total := amt * 10
		q := journalQuestion(
			fmt.Sprintf("fa-purch-%03d", i+1),
			"fixed_asset",
			fmt.Sprintf("備品 %s を購入し、代金は現金で支払った。", amtStr),
			"固定資産の取得は、借方に資産科目 (備品)、貸方に現金で処理する。",
			[]domain.JournalEntry{{Account: "備品", Amount: total}},
			[]domain.JournalEntry{{Account: "現金", Amount: total}},
			"core_300", "journal_240",
		)
		out = append(out, q)
	}
	return out
}

func loansAndBorrow(n int) []Question {
	out := make([]Question, 0, n)
	half := n / 2
	for i, amt := range amounts(half) {
		out = append(out, journalQuestion(
			fmt.Sprintf("loan-out-%03d", i+1),
			"loans",
			fmt.Sprintf("取引先に %s を貸し付け、現金を渡した。", formatYen(amt*10)),
			"貸付金は資産。借方に貸付金、貸方に現金。",
			[]domain.JournalEntry{{Account: "貸付金", Amount: amt * 10}},
			[]domain.JournalEntry{{Account: "現金", Amount: amt * 10}},
			"core_300", "journal_240",
		))
	}
	for i, amt := range amounts(n - half) {
		out = append(out, journalQuestion(
			fmt.Sprintf("loan-in-%03d", i+1),
			"loans",
			fmt.Sprintf("銀行から %s を借り入れ、現金を受け取った。", formatYen(amt*10)),
			"借入金は負債。借方に現金、貸方に借入金。",
			[]domain.JournalEntry{{Account: "現金", Amount: amt * 10}},
			[]domain.JournalEntry{{Account: "借入金", Amount: amt * 10}},
			"core_300", "journal_240",
		))
	}
	return out
}

func otherRecv(n int) []Question {
	out := make([]Question, 0, n)
	for i, amt := range amounts(n) {
		out = append(out, journalQuestion(
			fmt.Sprintf("other-adv-%03d", i+1),
			"other_recv",
			fmt.Sprintf("従業員に %s を立替払いした。", formatYen(amt)),
			"立替金は資産 (将来の返済請求)。借方に立替金、貸方に現金。",
			[]domain.JournalEntry{{Account: "立替金", Amount: amt}},
			[]domain.JournalEntry{{Account: "現金", Amount: amt}},
			"core_300", "journal_240",
		))
	}
	return out
}

func taxQuestions(n int) []Question {
	out := make([]Question, 0, n)
	for i, amt := range amounts(n) {
		q := journalQuestion(
			fmt.Sprintf("tax-cons-%03d", i+1),
			"tax",
			fmt.Sprintf("商品 %s (税抜) を売り上げ、消費税 10%% を含めて現金で受け取った (税抜方式)。", formatYen(amt*100)),
			"税抜方式: 借方に現金 (税込)、貸方に売上 (税抜) と仮受消費税。",
			[]domain.JournalEntry{{Account: "現金", Amount: amt * 110}},
			[]domain.JournalEntry{
				{Account: "売上", Amount: amt * 100},
				{Account: "仮受消費税", Amount: amt * 10},
			},
			"core_300", "comprehensive_300",
		)
		q.Difficulty = 2
		out = append(out, q)
	}
	return out
}

func wagesQuestions(n int) []Question {
	out := make([]Question, 0, n)
	for i, amt := range amounts(n) {
		gross := amt * 200
		withhold := gross / 10
		net := gross - withhold
		q := journalQuestion(
			fmt.Sprintf("wages-%03d", i+1),
			"wages",
			fmt.Sprintf("給料 %s から所得税 %s を差し引いた残額を現金で支払った。",
				formatYen(gross), formatYen(withhold)),
			"借方に給料 (総額)、貸方に預り金 (源泉税) と現金 (手取額) を計上する。",
			[]domain.JournalEntry{{Account: "給料", Amount: gross}},
			[]domain.JournalEntry{
				{Account: "預り金", Amount: withhold},
				{Account: "現金", Amount: net},
			},
			"core_300", "comprehensive_300",
		)
		q.Difficulty = 2
		out = append(out, q)
	}
	return out
}

func capitalQuestions(n int) []Question {
	out := make([]Question, 0, n)
	for i, amt := range amounts(n) {
		out = append(out, journalQuestion(
			fmt.Sprintf("capital-in-%03d", i+1),
			"capital",
			fmt.Sprintf("店主が現金 %s を元入れした (出資)。", formatYen(amt*100)),
			"資本元入れは、借方に現金、貸方に資本金で処理する。",
			[]domain.JournalEntry{{Account: "現金", Amount: amt * 100}},
			[]domain.JournalEntry{{Account: "資本金", Amount: amt * 100}},
			"core_300", "journal_240",
		))
	}
	return out
}

func adjustmentQuestions(n int) []Question {
	out := make([]Question, 0, n)
	for i := range n {
		baseAmt := int64((i + 1) * 5000)
		acq := baseAmt * 12
		dep := acq / 6 // 6 年 残存ゼロ定額
		q := journalQuestion(
			fmt.Sprintf("adj-dep-%03d", i+1),
			"adjustment",
			fmt.Sprintf("決算整理: 備品 (取得原価 %s、耐用年数 6 年、残存価額ゼロ、定額法) の減価償却を行う。",
				formatYen(acq)),
			"定額法の減価償却費 = 取得原価 / 耐用年数。借方に減価償却費、貸方に備品減価償却累計額 (間接法)。",
			[]domain.JournalEntry{{Account: "減価償却費", Amount: dep}},
			[]domain.JournalEntry{{Account: "備品減価償却累計額", Amount: dep}},
			"core_300", "comprehensive_300",
		)
		q.Difficulty = 2
		out = append(out, q)
	}
	return out
}

func trialBalanceQuestions(n int) []Question {
	out := make([]Question, 0, n)
	for i := range n {
		q := Question{
			Code:         fmt.Sprintf("tb-%03d", i+1),
			TopicCode:    "trial_balance",
			QuestionType: string(domain.QuestionTypeTrialBalance),
			Difficulty:   2,
			Prompt:       "試算表上、借方合計と貸方合計が一致しない場合に最初に確認すべきは何か。",
			Payload:      map[string]any{"choices": []string{"A: 元帳の集計ミス", "B: 仕訳の貸借不一致", "C: 期末棚卸残高"}},
			Answer:       domain.AnswerPayload{Type: domain.QuestionTypeTrialBalance, Choice: "B"},
			Explanation:  "試算表の不一致は、最も多くが仕訳段階での貸借金額ミスに起因する。次いで転記ミス、集計ミスの順で調査する。",
			References:   []string{"日商簿記3級 公式出題範囲 (kentei.ne.jp)"},
			Sets:         []string{"comprehensive_300"},
		}
		out = append(out, q)
	}
	return out
}

func worksheetQuestions(n int) []Question {
	out := make([]Question, 0, n)
	for i := range n {
		q := Question{
			Code:         fmt.Sprintf("ws-%03d", i+1),
			TopicCode:    "worksheet",
			QuestionType: string(domain.QuestionTypeWorksheet),
			Difficulty:   2,
			Prompt:       "精算表で、損益計算書欄の借方合計と貸方合計の差額は何を意味するか。",
			Payload:      map[string]any{"choices": []string{"A: 当期純利益または当期純損失", "B: 売上総利益", "C: 翌期繰越額"}},
			Answer:       domain.AnswerPayload{Type: domain.QuestionTypeWorksheet, Choice: "A"},
			Explanation:  "損益欄の差額は当期純利益 (貸方>借方) または当期純損失 (借方>貸方) を示し、貸借対照表欄の繰越利益剰余金に振り替えられる。",
			References:   []string{"日商簿記3級 公式出題範囲 (kentei.ne.jp)"},
			Sets:         []string{"comprehensive_300"},
		}
		out = append(out, q)
	}
	return out
}

func slipQuestions(n int) []Question {
	out := make([]Question, 0, n)
	for i := range n {
		q := Question{
			Code:         fmt.Sprintf("slip-%03d", i+1),
			TopicCode:    "slip",
			QuestionType: string(domain.QuestionTypeSubbook),
			Difficulty:   1,
			Prompt:       "3 伝票制 (入金・出金・振替) において、現金売上を起票する伝票はどれか。",
			Payload:      map[string]any{"choices": []string{"A: 入金伝票", "B: 出金伝票", "C: 振替伝票"}},
			Answer:       domain.AnswerPayload{Type: domain.QuestionTypeSubbook, Choice: "A"},
			Explanation:  "現金収入を伴う取引は入金伝票で起票する。出金伝票は現金支出、振替伝票はそれ以外。",
			References:   []string{"日商簿記3級 公式出題範囲 (kentei.ne.jp)"},
			Sets:         []string{"core_300"},
		}
		out = append(out, q)
	}
	return out
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
