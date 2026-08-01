package seed

import (
	"fmt"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// notesLoanPatterns は手形・電子記録債権債務・貸付借入の文型群。
func notesLoanPatterns() []pattern {
	return []pattern{
		{
			prefix: "nt-recv-collect", topic: "notes", diff: 1, n: 3,
			expl: "売掛金の回収として約束手形を受け取ったときは、受取手形 (資産) に振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 100000, 180000, 140000)
				return fmt.Sprintf("売掛金 %s の回収として、得意先振出しの約束手形を受け取った。", formatYen(b)),
					side(je("受取手形", b)), side(je("売掛金", b))
			},
		},
		{
			prefix: "nt-sale", topic: "notes", diff: 1, n: 3,
			expl: "手形による売上は、借方に受取手形、貸方に売上を計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 70000, 130000, 210000)
				return fmt.Sprintf("商品 %s を売り上げ、代金として約束手形を受け取った。", formatYen(b)),
					side(je("受取手形", b)), side(je("売上", b))
			},
		},
		{
			prefix: "nt-mature-in", topic: "notes", diff: 1, n: 3,
			expl: "受取手形の満期日に取立てが完了すると、当座預金が増加し受取手形が減少する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 100000, 180000, 140000)
				return fmt.Sprintf("かねて受け取っていた約束手形 %s が満期日に決済され、当座預金口座に入金された。", formatYen(b)),
					side(je("当座預金", b)), side(je("受取手形", b))
			},
		},
		{
			prefix: "nt-issue-payable", topic: "notes", diff: 1, n: 3,
			expl: "買掛金の支払いとして約束手形を振り出したときは、支払手形 (負債) に振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 60000, 110000, 85000)
				return fmt.Sprintf("買掛金 %s の支払いのため、約束手形を振り出した。", formatYen(b)),
					side(je("買掛金", b)), side(je("支払手形", b))
			},
		},
		{
			prefix: "nt-purch", topic: "notes", diff: 1, n: 3,
			expl: "手形による仕入は、借方に仕入、貸方に支払手形を計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 90000, 150000, 120000)
				return fmt.Sprintf("商品 %s を仕入れ、代金として約束手形を振り出した。", formatYen(b)),
					side(je("仕入", b)), side(je("支払手形", b))
			},
		},
		{
			prefix: "nt-mature-out", topic: "notes", diff: 1, n: 3,
			expl: "支払手形の満期日に当座預金から決済されると、負債 (支払手形) が減少する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 60000, 110000, 85000)
				return fmt.Sprintf("かねて振り出していた約束手形 %s が満期日に当座預金口座から決済された。", formatYen(b)),
					side(je("支払手形", b)), side(je("当座預金", b))
			},
		},
		{
			prefix: "nt-erec-occur", topic: "notes", diff: 2, n: 3,
			expl: "売掛金について電子記録債権の発生記録が行われると、売掛金から電子記録債権 (資産) へ振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 90000, 150000, 120000)
				return fmt.Sprintf("売掛金 %s について、取引銀行を通じて電子記録債権の発生記録が行われた。", formatYen(b)),
					side(je("電子記録債権", b)), side(je("売掛金", b))
			},
		},
		{
			prefix: "nt-erec-settle", topic: "notes", diff: 2, n: 3,
			expl: "電子記録債権が決済されると、普通預金が増加し電子記録債権が減少する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 90000, 150000, 120000)
				return fmt.Sprintf("電子記録債権 %s の支払期日が到来し、普通預金口座に入金された。", formatYen(b)),
					side(je("普通預金", b)), side(je("電子記録債権", b))
			},
		},
		{
			prefix: "nt-edebt-occur", topic: "notes", diff: 2, n: 3,
			expl: "買掛金について電子記録債務の発生記録が行われると、買掛金から電子記録債務 (負債) へ振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 75000, 125000, 100000)
				return fmt.Sprintf("買掛金 %s について、電子記録債務の発生記録が行われた。", formatYen(b)),
					side(je("買掛金", b)), side(je("電子記録債務", b))
			},
		},
		{
			prefix: "nt-edebt-settle", topic: "notes", diff: 2, n: 3,
			expl: "電子記録債務の決済により、負債が減少し当座預金が減少する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 75000, 125000, 100000)
				return fmt.Sprintf("電子記録債務 %s の支払期日が到来し、当座預金口座から決済された。", formatYen(b)),
					side(je("電子記録債務", b)), side(je("当座預金", b))
			},
		},
		{
			prefix: "ln-lend", topic: "loans", diff: 1, n: 3,
			expl: "貸付けは将来返済を受ける権利であり、貸付金 (資産) を計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 200000, 500000, 350000)
				return fmt.Sprintf("取引先に %s を貸し付け、現金を渡した。", formatYen(b)),
					side(je("貸付金", b)), side(je("現金", b))
			},
		},
		{
			prefix: "ln-collect-int", topic: "loans", diff: 2, n: 3,
			expl: "貸付金の回収と利息の受取り。元金は貸付金の減少、利息は受取利息 (収益) とする。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 200000, 500000, 350000)
				interest := b * 2 / 100
				return fmt.Sprintf("貸付金 %s を利息 (年利率 2%%、貸付期間 1 年) とともに回収し、普通預金口座に入金された。", formatYen(b)),
					side(je("普通預金", b+interest)), side(je("貸付金", b), je("受取利息", interest))
			},
		},
		{
			prefix: "ln-borrow", topic: "loans", diff: 1, n: 3,
			expl: "借入れは将来返済する義務であり、借入金 (負債) を計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 300000, 800000, 500000)
				return fmt.Sprintf("銀行から %s を借り入れ、当座預金口座に入金された。", formatYen(b)),
					side(je("当座預金", b)), side(je("借入金", b))
			},
		},
		{
			prefix: "ln-repay-int", topic: "loans", diff: 2, n: 3,
			expl: "借入金の返済と利息の支払い。元金は借入金の減少、利息は支払利息 (費用) とする。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 300000, 800000, 500000)
				interest := b * 3 / 100
				return fmt.Sprintf("借入金 %s を利息 (年利率 3%%、借入期間 1 年) とともに当座預金口座から返済した。", formatYen(b)),
					side(je("借入金", b), je("支払利息", interest)), side(je("当座預金", b+interest))
			},
		},
		{
			prefix: "ln-note-lend", topic: "loans", diff: 2, n: 3,
			expl: "借用証書の代わりに約束手形を受け取って貸し付けたときは、手形貸付金 (資産) を用いる。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 150000, 400000, 270000)
				return fmt.Sprintf("取引先に %s を貸し付け、同額の約束手形を受け取り、現金を渡した。", formatYen(b)),
					side(je("手形貸付金", b)), side(je("現金", b))
			},
		},
		{
			prefix: "ln-note-borrow", topic: "loans", diff: 2, n: 3,
			expl: "約束手形を振り出して借り入れたときは、手形借入金 (負債) を用いる。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 250000, 600000, 420000)
				return fmt.Sprintf("銀行から %s を借り入れ、同額の約束手形を振り出し、現金を受け取った。", formatYen(b)),
					side(je("現金", b)), side(je("手形借入金", b))
			},
		},
		{
			prefix: "ln-int-month", topic: "loans", diff: 2, n: 3,
			expl: "利息の月割計算: 元金 × 年利率 × 経過月数 / 12。500,000 円 × 2.4% × 5/12 = 5,000 円。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 500000, 1000000, 800000)
				interest := b / 100 // 年利 2.4% × 5/12 = 元金の 1%
				return fmt.Sprintf("借入金 %s (年利率 2.4%%) について、5 か月分の利息を現金で支払った。", formatYen(b)),
					side(je("支払利息", interest)), side(je("現金", interest))
			},
		},
	}
}
