package seed_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/seed"
)

// entriesKey は仕訳エントリ集合の順序非依存の文字列表現。
func entriesKey(es []domain.JournalEntry) string {
	parts := make([]string, 0, len(es))
	for _, e := range es {
		parts = append(parts, fmt.Sprintf("%s=%d", e.Account, e.Amount))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// 代表文型のゴールデン仕訳。期待値は簿記 3 級の標準処理を手計算で検証したもの。
// 文型の会計ロジック (派生額の比率・貸借構成) の退行をここで検出する。
func TestBankGoldenJournals(t *testing.T) {
	t.Parallel()
	byCode := map[string]seed.Question{}
	for _, q := range seed.Generate() {
		byCode[q.Code] = q
	}
	cases := []struct {
		code    string
		debits  string
		credits string
	}{
		// クレジット売上: 手数料 4% を販売時に計上
		{"ms-cc-sale-001", "クレジット売掛金=48000,支払手数料=2000", "売上=50000"},
		// 現金過不足の原因判明。不足分は旅費交通費とする
		{"ca-short-cause-001", "旅費交通費=3000", "現金過不足=3000"},
		// 前期発生売掛金の貸倒れ: 引当金残高を超える分は貸倒損失
		{"rc-writeoff-part-001", "貸倒引当金=30000,貸倒損失=20000", "売掛金=50000"},
		// 期首売却・売却益: 簿価 120,000 の備品を 150,000 で売却
		{"fa-sell-gain-001", "備品減価償却累計額=180000,現金=150000", "備品=300000,固定資産売却益=30000"},
		// 消費税の決算整理: 仮受 50,000 − 仮払 30,000 = 未払消費税 20,000
		{"tx-cons-close-001", "仮受消費税=50000", "仮払消費税=30000,未払消費税=20000"},
		// 給料支払: 所得税・社会保険料を控除した残額を現金払い
		{"wg-pay-social-001", "給料=300000", "所得税預り金=12000,現金=258000,社会保険料預り金=30000"},
		// 剰余金の配当: 配当 200,000 + 利益準備金 10 分の 1 積立
		{"cp-dividend-001", "繰越利益剰余金=220000", "利益準備金=20000,未払配当金=200000"},
		// 月割減価償却: 480,000 / 8 年 × 6/12 = 30,000
		{"ad-dep-month-001", "減価償却費=30000", "備品減価償却累計額=30000"},
		// 現金過不足の決算処理: 判明分は通信費、残額は雑損
		{"ad-short-close-001", "通信費=3000,雑損=2000", "現金過不足=5000"},
		// 借入金利息の月割: 500,000 × 年 2.4% × 5/12 = 5,000
		{"ln-int-month-001", "支払利息=5000", "現金=5000"},
	}
	for _, c := range cases {
		q, ok := byCode[c.code]
		if !ok {
			t.Fatalf("golden question %s not found in bank", c.code)
		}
		if got := entriesKey(q.Answer.Debits); got != c.debits {
			t.Fatalf("%s debits = %s, want %s", c.code, got, c.debits)
		}
		if got := entriesKey(q.Answer.Credits); got != c.credits {
			t.Fatalf("%s credits = %s, want %s", c.code, got, c.credits)
		}
	}
}
