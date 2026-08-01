package seed

import (
	"fmt"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// merchPatterns は商品売買 (三分法) の文型群。
func merchPatterns() []pattern {
	return []pattern{
		{
			prefix: "ms-cash-sale", topic: "merch_sale", diff: 1, n: 3,
			expl: "現金売上の取引である。借方は資産の増加 (現金)、貸方は収益の発生 (売上)。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 28000, 61000, 97000)
				return fmt.Sprintf("商品 %s を売り上げ、代金は現金で受け取った。", formatYen(b)),
					side(je("現金", b)), side(je("売上", b))
			},
		},
		{
			prefix: "ms-cash-purch", topic: "merch_sale", diff: 1, n: 3,
			expl: "現金仕入の取引である。三分法では借方に費用 (仕入)、貸方に資産の減少 (現金) を計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 35000, 82000, 149000)
				return fmt.Sprintf("商品 %s を仕入れ、代金は現金で支払った。", formatYen(b)),
					side(je("仕入", b)), side(je("現金", b))
			},
		},
		{
			prefix: "ms-credit-sale", topic: "merch_sale", diff: 1, n: 3,
			expl: "掛売上では、借方に売掛金 (後日代金を受け取る権利)、貸方に売上を計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 45000, 130000, 260000)
				return fmt.Sprintf("商品 %s を売り上げ、代金は掛けとした。", formatYen(b)),
					side(je("売掛金", b)), side(je("売上", b))
			},
		},
		{
			prefix: "ms-credit-purch", topic: "merch_sale", diff: 1, n: 3,
			expl: "掛仕入では、借方に仕入、貸方に買掛金 (後日代金を支払う義務) を計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 52000, 88000, 174000)
				return fmt.Sprintf("商品 %s を仕入れ、代金は掛けとした。", formatYen(b)),
					side(je("仕入", b)), side(je("買掛金", b))
			},
		},
		{
			prefix: "ms-sale-return", topic: "merch_sale", diff: 2, n: 2,
			expl: "売上戻り (返品) は売上の取消しである。売上を借方に立て、売掛金を減らす (逆仕訳)。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 8000, 15000)
				return fmt.Sprintf("掛けで売り上げた商品のうち %s 分が品違いのため返品された。", formatYen(b)),
					side(je("売上", b)), side(je("売掛金", b))
			},
		},
		{
			prefix: "ms-purch-return", topic: "merch_sale", diff: 2, n: 2,
			expl: "仕入戻し (返品) は仕入の取消しである。買掛金を借方に立て、仕入を減らす (逆仕訳)。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 6000, 12000)
				return fmt.Sprintf("掛けで仕入れた商品のうち %s 分を品質不良のため返品した。", formatYen(b)),
					side(je("買掛金", b)), side(je("仕入", b))
			},
		},
		{
			prefix: "ms-adv-pay", topic: "merch_sale", diff: 2, n: 2,
			expl: "商品代金の内金 (手付金) の支払いは、商品を受け取る権利として前払金 (資産) を計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 20000, 50000)
				return fmt.Sprintf("商品の注文にあたり、内金として %s を現金で支払った。", formatYen(b)),
					side(je("前払金", b)), side(je("現金", b))
			},
		},
		{
			prefix: "ms-purch-adv", topic: "merch_sale", diff: 2, n: 3,
			expl: "注文時に支払った前払金を仕入代金に充当し、残額を買掛金とする。借方は仕入 (総額)。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 50000, 120000, 85000)
				adv := b / 5
				return fmt.Sprintf("商品 %s を仕入れ、注文時に支払った内金 %s を差し引いた残額を掛けとした。",
						formatYen(b), formatYen(adv)),
					side(je("仕入", b)), side(je("前払金", adv), je("買掛金", b-adv))
			},
		},
		{
			prefix: "ms-adv-recv", topic: "merch_sale", diff: 2, n: 2,
			expl: "商品代金の内金の受取りは、商品を引き渡す義務として前受金 (負債) を計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 30000, 70000)
				return fmt.Sprintf("商品の注文を受け、内金として %s を現金で受け取った。", formatYen(b)),
					side(je("現金", b)), side(je("前受金", b))
			},
		},
		{
			prefix: "ms-sale-adv", topic: "merch_sale", diff: 2, n: 3,
			expl: "受注時の前受金を売上代金に充当し、残額を売掛金とする。貸方は売上 (総額)。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 60000, 150000, 90000)
				adv := b / 5
				return fmt.Sprintf("商品 %s を引き渡した。代金のうち受注時に受け取った内金 %s を充当し、残額は掛けとした。",
						formatYen(b), formatYen(adv)),
					side(je("前受金", adv), je("売掛金", b-adv)), side(je("売上", b))
			},
		},
		{
			prefix: "ms-cc-sale", topic: "merch_sale", diff: 2, n: 3,
			expl: "クレジット払いの売上は、信販会社に対する債権としてクレジット売掛金を計上し、手数料は販売時に支払手数料 (費用) とする。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 50000, 200000, 125000)
				fee := b * 4 / 100
				return fmt.Sprintf("商品 %s をクレジット払いの条件で販売した。信販会社への手数料 (販売代金の 4%%) は販売時に計上する。",
						formatYen(b)),
					side(je("クレジット売掛金", b-fee), je("支払手数料", fee)), side(je("売上", b))
			},
		},
		{
			prefix: "ms-ship-ours", topic: "merch_sale", diff: 2, n: 2,
			expl: "当方負担の発送費は費用 (発送費) として計上する。売掛金は商品代金のみ。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 80000, 140000)
				const f = 2000
				return fmt.Sprintf("商品 %s を掛けで売り上げ、当方負担の発送費 %s を現金で支払った。",
						formatYen(b), formatYen(f)),
					side(je("売掛金", b), je("発送費", f)), side(je("売上", b), je("現金", f))
			},
		},
		{
			prefix: "ms-gift-cert", topic: "merch_sale", diff: 2, n: 2,
			expl: "自治体などが発行する商品券を受け取ったときは、後日精算を請求できる権利として受取商品券 (資産) を計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 15000, 40000)
				return fmt.Sprintf("商品 %s を売り上げ、代金として自治体発行の商品券を受け取った。", formatYen(b)),
					side(je("受取商品券", b)), side(je("売上", b))
			},
		},
		{
			prefix: "ms-half-cash", topic: "merch_sale", diff: 2, n: 3,
			expl: "代金の一部を現金、残額を掛けとする複合仕訳。借方は現金と売掛金の 2 科目になる。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 90000, 160000, 230000)
				half := b / 2
				return fmt.Sprintf("商品 %s を売り上げ、代金のうち半額は現金で受け取り、残額は掛けとした。", formatYen(b)),
					side(je("現金", half), je("売掛金", b-half)), side(je("売上", b))
			},
		},
	}
}
