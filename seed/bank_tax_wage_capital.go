package seed

import (
	"fmt"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// taxWageCapitalPatterns は税金・給与・資本 (株式会社) の文型群。
func taxWageCapitalPatterns() []pattern {
	return []pattern{
		{
			prefix: "tx-cons-purch", topic: "tax", diff: 2, n: 2,
			expl: "税抜方式の仕入では、本体価格を仕入、消費税額を仮払消費税 (資産) として区分する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 40000, 90000)
				tax := b / 10
				return fmt.Sprintf("商品 %s (税抜) を仕入れ、消費税 10%% を含めた代金を現金で支払った (税抜方式)。", formatYen(b)),
					side(je("仕入", b), je("仮払消費税", tax)), side(je("現金", b+tax))
			},
		},
		{
			prefix: "tx-cons-sale", topic: "tax", diff: 2, n: 2,
			expl: "税抜方式の売上では、本体価格を売上、預かった消費税額を仮受消費税 (負債) として区分する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 60000, 120000)
				tax := b / 10
				return fmt.Sprintf("商品 %s (税抜) を売り上げ、消費税 10%% を含めた代金を現金で受け取った (税抜方式)。", formatYen(b)),
					side(je("現金", b+tax)), side(je("売上", b), je("仮受消費税", tax))
			},
		},
		{
			prefix: "tx-cons-close", topic: "tax", diff: 2, n: 2,
			expl: "決算で仮受消費税と仮払消費税を相殺し、差額を未払消費税 (納付義務) とする。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				x := pick(i, 50000, 80000)
				y := pick(i, 30000, 52000)
				return fmt.Sprintf("決算整理: 仮受消費税の残高は %s、仮払消費税の残高は %s である。両者を相殺し、納付額を未払消費税に計上する (税抜方式)。",
						formatYen(x), formatYen(y)),
					side(je("仮受消費税", x)), side(je("仮払消費税", y), je("未払消費税", x-y))
			},
		},
		{
			prefix: "tx-cons-pay", topic: "tax", diff: 1, n: 2,
			expl: "確定申告により未払消費税を納付したときは、負債の減少として処理する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 20000, 28000)
				return fmt.Sprintf("確定申告を行い、未払消費税 %s を現金で納付した。", formatYen(b)),
					side(je("未払消費税", b)), side(je("現金", b))
			},
		},
		{
			prefix: "tx-corp-interim", topic: "tax", diff: 2, n: 2,
			expl: "法人税等の中間納付額は、確定額が決まるまで仮払法人税等 (資産) として処理する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 150000, 240000)
				return fmt.Sprintf("法人税等の中間申告を行い、%s を当座預金口座から納付した。", formatYen(b)),
					side(je("仮払法人税等", b)), side(je("当座預金", b))
			},
		},
		{
			prefix: "tx-corp-close", topic: "tax", diff: 2, n: 2,
			expl: "決算で確定した法人税等から中間納付額 (仮払法人税等) を差し引き、残額を未払法人税等とする。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				t := pick(i, 400000, 600000)
				interim := pick(i, 150000, 240000)
				return fmt.Sprintf("決算により当期の法人税等が %s と確定した。中間納付額 %s を控除した残額を未払法人税等に計上する (法人税等勘定を用いること)。",
						formatYen(t), formatYen(interim)),
					side(je("法人税等", t)), side(je("仮払法人税等", interim), je("未払法人税等", t-interim))
			},
		},
		{
			prefix: "tx-corp-pay", topic: "tax", diff: 1, n: 2,
			expl: "確定申告により未払法人税等を納付したときは、負債の減少として処理する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 250000, 360000)
				return fmt.Sprintf("確定申告を行い、未払法人税等 %s を普通預金口座から納付した。", formatYen(b)),
					side(je("未払法人税等", b)), side(je("普通預金", b))
			},
		},
		{
			prefix: "tx-property", topic: "tax", diff: 1, n: 2,
			expl: "固定資産税は費用としての税金であり、租税公課で処理する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 45000, 60000)
				return fmt.Sprintf("固定資産税の納税通知書を受け取り、%s を現金で納付した。", formatYen(b)),
					side(je("租税公課", b)), side(je("現金", b))
			},
		},
		{
			prefix: "tx-stamp", topic: "tax", diff: 1, n: 2,
			expl: "収入印紙の購入額は租税公課 (費用) で処理する (未使用分は決算で貯蔵品へ振替)。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 4000, 10000)
				return fmt.Sprintf("収入印紙 %s を現金で購入し、直ちに使用した。", formatYen(b)),
					side(je("租税公課", b)), side(je("現金", b))
			},
		},
		{
			prefix: "wg-pay", topic: "wages", diff: 2, n: 2,
			expl: "給料総額を費用計上し、源泉徴収した所得税は所得税預り金 (負債)、差引残額を従業員へ支払う。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				g := pick(i, 300000, 450000)
				s := pick(i, 15000, 24000)
				return fmt.Sprintf("給料 %s の支払いにあたり、所得税の源泉徴収額 %s を差し引いた残額を普通預金口座から振り込んだ (所得税預り金勘定を用いること)。",
						formatYen(g), formatYen(s)),
					side(je("給料", g)), side(je("所得税預り金", s), je("普通預金", g-s))
			},
		},
		{
			prefix: "wg-pay-social", topic: "wages", diff: 2, n: 2,
			expl: "所得税と社会保険料の従業員負担分をそれぞれ預り金として区分し、差引残額を現金で支払う。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				g := pick(i, 300000, 500000)
				s := pick(i, 12000, 25000)
				h := pick(i, 30000, 50000)
				return fmt.Sprintf("給料 %s の支払いにあたり、所得税の源泉徴収額 %s と社会保険料の従業員負担分 %s を差し引いた残額を現金で支払った (所得税預り金勘定・社会保険料預り金勘定を用いること)。",
						formatYen(g), formatYen(s), formatYen(h)),
					side(je("給料", g)),
					side(je("所得税預り金", s), je("社会保険料預り金", h), je("現金", g-s-h))
			},
		},
		{
			prefix: "wg-remit-tax", topic: "wages", diff: 1, n: 2,
			expl: "源泉徴収した所得税の納付は、預り金 (負債) の減少として処理する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 15000, 24000)
				return fmt.Sprintf("従業員の給料から源泉徴収していた所得税 %s を税務署に現金で納付した (所得税預り金勘定を用いること)。", formatYen(b)),
					side(je("所得税預り金", b)), side(je("現金", b))
			},
		},
		{
			prefix: "wg-social-pay", topic: "wages", diff: 2, n: 2,
			expl: "社会保険料の納付では、従業員負担分は社会保険料預り金の減少、会社負担分は法定福利費 (費用) とする。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				h := pick(i, 30000, 50000)
				return fmt.Sprintf("社会保険料 %s (従業員負担分 %s、会社負担分 %s) を普通預金口座から納付した。",
						formatYen(2*h), formatYen(h), formatYen(h)),
					side(je("社会保険料預り金", h), je("法定福利費", h)), side(je("普通預金", 2*h))
			},
		},
		{
			prefix: "wg-offset-advance", topic: "wages", diff: 2, n: 2,
			expl: "従業員への立替金は給料支払時に差し引いて回収する。借方は給料 (総額)。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				g := pick(i, 280000, 350000)
				a := pick(i, 20000, 30000)
				return fmt.Sprintf("給料 %s の支払いにあたり、かねて立替払いしていた %s を差し引き、残額を現金で支払った。",
						formatYen(g), formatYen(a)),
					side(je("給料", g)), side(je("立替金", a), je("現金", g-a))
			},
		},
		{
			prefix: "cp-founding", topic: "capital", diff: 2, n: 2,
			expl: "株式会社の設立時は、払込金額の全額を資本金とするのが原則である。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				shares := pick(i, 100, 200)
				price := pick(i, 50000, 30000)
				total := shares * price
				return fmt.Sprintf("会社の設立にあたり株式 %d 株を 1 株 %s で発行し、払込金の全額が当座預金口座に入金された。払込金額の全額を資本金とする。",
						shares, formatYen(price)),
					side(je("当座預金", total)), side(je("資本金", total))
			},
		},
		{
			prefix: "cp-increase", topic: "capital", diff: 2, n: 2,
			expl: "増資による払込金も、原則として全額を資本金に計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 3000000, 4500000)
				return fmt.Sprintf("増資のため新株を発行し、払込金 %s が普通預金口座に入金された。払込金額の全額を資本金とする。", formatYen(b)),
					side(je("普通預金", b)), side(je("資本金", b))
			},
		},
		{
			prefix: "cp-dividend", topic: "capital", diff: 2, n: 2,
			expl: "剰余金の配当を決議したときは、繰越利益剰余金を減らし、未払配当金 (負債) と利益準備金 (配当額の 10 分の 1) を計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				d := pick(i, 200000, 300000)
				reserve := d / 10
				return fmt.Sprintf("株主総会で、繰越利益剰余金から配当金 %s の支払いと利益準備金 %s の積立てを決議した。",
						formatYen(d), formatYen(reserve)),
					side(je("繰越利益剰余金", d+reserve)),
					side(je("未払配当金", d), je("利益準備金", reserve))
			},
		},
		{
			prefix: "cp-dividend-pay", topic: "capital", diff: 1, n: 2,
			expl: "配当金の支払いにより未払配当金 (負債) が減少する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 200000, 300000)
				return fmt.Sprintf("株主総会で決議していた配当金 %s を当座預金口座から支払った。", formatYen(b)),
					side(je("未払配当金", b)), side(je("当座預金", b))
			},
		},
		{
			prefix: "cp-profit-close", topic: "capital", diff: 2, n: 2,
			expl: "決算振替: 損益勘定で算定した当期純利益は、繰越利益剰余金 (純資産) へ振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 350000, 800000)
				return fmt.Sprintf("決算の結果、当期純利益 %s を損益勘定から繰越利益剰余金勘定へ振り替えた。", formatYen(b)),
					side(je("損益", b)), side(je("繰越利益剰余金", b))
			},
		},
		{
			prefix: "cp-loss-close", topic: "capital", diff: 2, n: 2,
			expl: "当期純損失は繰越利益剰余金を減らす方向へ振り替える (借方: 繰越利益剰余金)。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 120000, 250000)
				return fmt.Sprintf("決算の結果、当期純損失 %s を損益勘定から繰越利益剰余金勘定へ振り替えた。", formatYen(b)),
					side(je("繰越利益剰余金", b)), side(je("損益", b))
			},
		},
	}
}
