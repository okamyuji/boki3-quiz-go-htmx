package seed

import (
	"fmt"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// fixedOtherPatterns は固定資産とその他債権債務の文型群。
func fixedOtherPatterns() []pattern {
	return []pattern{
		{
			prefix: "fa-equip-incid", topic: "fixed_asset", diff: 2, n: 3,
			expl: "固定資産の取得原価には引取運賃などの付随費用を含める。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 200000, 350000, 280000)
				f := pick(i, 5000, 8000, 7000)
				return fmt.Sprintf("備品 %s を購入し、引取運賃 %s とともに現金で支払った。", formatYen(b), formatYen(f)),
					side(je("備品", b+f)), side(je("現金", b+f))
			},
		},
		{
			prefix: "fa-land-incid", topic: "fixed_asset", diff: 2, n: 2,
			expl: "土地の取得原価には整地費用などの付随費用を含める。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 2000000, 3500000)
				f := pick(i, 100000, 150000)
				return fmt.Sprintf("土地 %s を購入し、整地費用 %s とともに小切手を振り出して支払った。", formatYen(b), formatYen(f)),
					side(je("土地", b+f)), side(je("当座預金", b+f))
			},
		},
		{
			prefix: "fa-building-unpaid", topic: "fixed_asset", diff: 1, n: 3,
			expl: "商品以外の資産を後払いで購入したときの債務は、買掛金ではなく未払金を用いる。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 1200000, 2400000, 1800000)
				return fmt.Sprintf("建物 %s を購入した。代金は翌月末に支払うこととした。", formatYen(b)),
					side(je("建物", b)), side(je("未払金", b))
			},
		},
		{
			prefix: "fa-sell-gain", topic: "fixed_asset", diff: 2, n: 3,
			expl: "間接法では減価償却累計額を借方に振り替えて備品を帳簿から消す。売価が帳簿価額を上回る分は固定資産売却益。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				c := pick(i, 300000, 480000, 600000)
				acc := c * 3 / 5
				price := c / 2
				gain := price - (c - acc) // = c/10
				return fmt.Sprintf("期首に、備品 (取得原価 %s、減価償却累計額 %s、間接法で記帳) を %s で売却し、代金は現金で受け取った。",
						formatYen(c), formatYen(acc), formatYen(price)),
					side(je("現金", price), je("備品減価償却累計額", acc)),
					side(je("備品", c), je("固定資産売却益", gain))
			},
		},
		{
			prefix: "fa-sell-loss", topic: "fixed_asset", diff: 2, n: 2,
			expl: "売価が帳簿価額 (取得原価 − 減価償却累計額) を下回る分は固定資産売却損 (費用) とする。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				c := pick(i, 400000, 640000)
				acc := c / 4
				price := c * 5 / 8
				loss := (c - acc) - price // = c/8
				return fmt.Sprintf("期首に、備品 (取得原価 %s、減価償却累計額 %s、間接法で記帳) を %s で売却し、代金は現金で受け取った。",
						formatYen(c), formatYen(acc), formatYen(price)),
					side(je("現金", price), je("備品減価償却累計額", acc), je("固定資産売却損", loss)),
					side(je("備品", c))
			},
		},
		{
			prefix: "fa-sell-unpaid", topic: "fixed_asset", diff: 2, n: 2,
			expl: "商品以外の資産を売却した未収の代金は、売掛金ではなく未収入金を用いる。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 1500000, 2000000)
				g := pick(i, 100000, 200000)
				return fmt.Sprintf("土地 (帳簿価額 %s) を %s で売却し、代金は月末に受け取ることとした。",
						formatYen(b), formatYen(b+g)),
					side(je("未収入金", b+g)), side(je("土地", b), je("固定資産売却益", g))
			},
		},
		{
			prefix: "fa-capex", topic: "fixed_asset", diff: 2, n: 2,
			expl: "資産価値を高める支出 (資本的支出) は建物の取得原価に加算し、現状維持のための支出 (収益的支出) は修繕費とする。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				x := pick(i, 800000, 500000)
				y := pick(i, 200000, 100000)
				return fmt.Sprintf("建物の改修を行い、代金 %s を小切手を振り出して支払った。このうち %s は耐震補強工事 (資本的支出)、%s は壁の塗替え (収益的支出) である。",
						formatYen(x+y), formatYen(x), formatYen(y)),
					side(je("建物", x), je("修繕費", y)), side(je("当座預金", x+y))
			},
		},
		{
			prefix: "fa-repair", topic: "fixed_asset", diff: 1, n: 3,
			expl: "機能維持のための修理代金は修繕費 (費用) として処理する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 30000, 70000, 50000)
				return fmt.Sprintf("窓ガラスの破損を修理し、代金 %s を現金で支払った。", formatYen(b)),
					side(je("修繕費", b)), side(je("現金", b))
			},
		},
		{
			prefix: "or-advance-emp", topic: "other_recv", diff: 1, n: 3,
			expl: "従業員負担分の立替払いは、返済を請求できる権利として立替金 (資産) を計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 12000, 20000, 16000)
				return fmt.Sprintf("従業員が負担すべき生命保険料 %s を現金で立替払いした。", formatYen(b)),
					side(je("立替金", b)), side(je("現金", b))
			},
		},
		{
			prefix: "or-advance-back", topic: "other_recv", diff: 1, n: 3,
			expl: "立替金の回収により資産が現金に振り替わる。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 12000, 20000, 16000)
				return fmt.Sprintf("従業員に立替払いしていた %s を現金で回収した。", formatYen(b)),
					side(je("現金", b)), side(je("立替金", b))
			},
		},
		{
			prefix: "or-tmp-pay", topic: "other_recv", diff: 1, n: 3,
			expl: "内容や金額が未確定の支出は仮払金 (資産) として処理し、確定後に精算する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 50000, 80000, 65000)
				return fmt.Sprintf("従業員の出張にあたり、旅費の概算額 %s を現金で前渡しした。", formatYen(b)),
					side(je("仮払金", b)), side(je("現金", b))
			},
		},
		{
			prefix: "or-tmp-settle", topic: "other_recv", diff: 2, n: 3,
			expl: "出張精算では確定した旅費交通費を計上し、概算払いとの差額 (残金) を現金で受け取る。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 50000, 80000, 65000)
				x := pick(i, 42000, 71000, 58000)
				return fmt.Sprintf("出張から戻った従業員が旅費を精算し、概算払いしていた %s のうち旅費交通費は %s と確定、残額は現金で返金された。",
						formatYen(b), formatYen(x)),
					side(je("旅費交通費", x), je("現金", b-x)), side(je("仮払金", b))
			},
		},
		{
			prefix: "or-tmp-recv", topic: "other_recv", diff: 2, n: 3,
			expl: "内容不明の入金は仮受金 (負債) として処理し、内容判明後に振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 60000, 100000, 80000)
				return fmt.Sprintf("出張中の従業員から当座預金口座に %s の振込みがあったが、その内容は不明である。", formatYen(b)),
					side(je("当座預金", b)), side(je("仮受金", b))
			},
		},
		{
			prefix: "or-tmp-clear", topic: "other_recv", diff: 2, n: 3,
			expl: "仮受金の内容が判明したら、本来の勘定 (売掛金の回収) へ振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 60000, 100000, 80000)
				return fmt.Sprintf("かねて仮受金として処理していた %s は、得意先からの売掛金の回収であることが判明した。", formatYen(b)),
					side(je("仮受金", b)), side(je("売掛金", b))
			},
		},
		{
			prefix: "or-supplies-unpaid", topic: "other_recv", diff: 1, n: 3,
			expl: "事務用消耗品は費用 (消耗品費)。商品以外の後払い債務は未払金を用いる。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 8000, 15000, 11000)
				return fmt.Sprintf("事務用の文房具 %s を購入し、代金は月末に支払うこととした。", formatYen(b)),
					side(je("消耗品費", b)), side(je("未払金", b))
			},
		},
		{
			prefix: "or-deposit-rent", topic: "other_recv", diff: 2, n: 2,
			expl: "敷金は返還請求権として差入保証金 (資産)、仲介手数料は支払手数料、家賃は支払家賃で処理する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				d := pick(i, 200000, 300000)
				f := pick(i, 100000, 150000)
				r := pick(i, 100000, 150000)
				return fmt.Sprintf("店舗の賃借契約を結び、敷金 %s、不動産会社への仲介手数料 %s、1 か月分の家賃 %s を現金で支払った。",
						formatYen(d), formatYen(f), formatYen(r)),
					side(je("差入保証金", d), je("支払手数料", f), je("支払家賃", r)),
					side(je("現金", d+f+r))
			},
		},
	}
}
