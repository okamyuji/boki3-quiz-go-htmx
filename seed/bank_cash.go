package seed

import (
	"fmt"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// cashPatterns は現金預金 (当座・小口現金・現金過不足) の文型群。
func cashPatterns() []pattern {
	return []pattern{
		{
			prefix: "ca-to-checking", topic: "cash", diff: 1, n: 3,
			expl: "現金から当座預金への預入れは資産間の振替。借方に当座預金、貸方に現金。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 100000, 250000, 430000)
				return fmt.Sprintf("現金 %s を当座預金口座に預け入れた。", formatYen(b)),
					side(je("当座預金", b)), side(je("現金", b))
			},
		},
		{
			prefix: "ca-withdraw", topic: "cash", diff: 1, n: 3,
			expl: "普通預金からの引出しは資産間の振替。借方に現金、貸方に普通預金。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 40000, 80000, 130000)
				return fmt.Sprintf("普通預金口座から現金 %s を引き出した。", formatYen(b)),
					side(je("現金", b)), side(je("普通預金", b))
			},
		},
		{
			prefix: "ca-transfer-fee", topic: "cash", diff: 2, n: 3,
			expl: "振込手数料 (当方負担) は支払手数料 (費用)。普通預金の減少額は振込額と手数料の合計になる。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 70000, 120000, 95000)
				const fee = 300
				return fmt.Sprintf("買掛金 %s を普通預金口座から振り込んで支払った。振込手数料 %s (当方負担) も同口座から支払われた。",
						formatYen(b), formatYen(fee)),
					side(je("買掛金", b), je("支払手数料", fee)), side(je("普通預金", b+fee))
			},
		},
		{
			prefix: "ca-short-found", topic: "cash", diff: 2, n: 3,
			expl: "実際有高が帳簿残高より少ないときは、不足額を現金過不足 (借方) に振り替えて帳簿を実際有高に合わせる。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 3000, 5000, 7000)
				return fmt.Sprintf("現金の実査を行ったところ、実際有高が帳簿残高より %s 少なかった。", formatYen(b)),
					side(je("現金過不足", b)), side(je("現金", b))
			},
		},
		{
			prefix: "ca-short-cause", topic: "cash", diff: 2, n: 3,
			expl: "不足の原因が判明したら、現金過不足から本来の費用勘定 (旅費交通費) へ振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 3000, 5000, 7000)
				return fmt.Sprintf("かねて借方に計上していた現金過不足 %s について、原因は旅費交通費の記帳漏れと判明した。", formatYen(b)),
					side(je("旅費交通費", b)), side(je("現金過不足", b))
			},
		},
		{
			prefix: "ca-over-found", topic: "cash", diff: 2, n: 3,
			expl: "実際有高が帳簿残高より多いときは、過剰額を現金過不足 (貸方) に振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 2000, 4000, 6000)
				return fmt.Sprintf("現金の実査を行ったところ、実際有高が帳簿残高より %s 多かった。", formatYen(b)),
					side(je("現金", b)), side(je("現金過不足", b))
			},
		},
		{
			prefix: "ca-over-cause", topic: "cash", diff: 2, n: 3,
			expl: "過剰の原因が判明したら、現金過不足から本来の収益勘定 (受取手数料) へ振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 2000, 4000, 6000)
				return fmt.Sprintf("かねて貸方に計上していた現金過不足 %s について、原因は受取手数料の記帳漏れと判明した。", formatYen(b)),
					side(je("現金過不足", b)), side(je("受取手数料", b))
			},
		},
		{
			prefix: "ca-petty-report", topic: "cash", diff: 2, n: 3,
			expl: "小口現金係からの支払報告により、各費用勘定を借方に計上し小口現金を減らす。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				x := pick(i, 6000, 12000, 9000)
				y := pick(i, 4000, 8000, 6000)
				return fmt.Sprintf("小口現金係から、旅費交通費 %s および消耗品費 %s の支払報告を受けた。",
						formatYen(x), formatYen(y)),
					side(je("旅費交通費", x), je("消耗品費", y)), side(je("小口現金", x+y))
			},
		},
		{
			prefix: "ca-petty-replenish", topic: "cash", diff: 2, n: 3,
			expl: "定額資金前渡制 (インプレスト・システム) では、使用額と同額を補給して小口現金を定額に戻す。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 15000, 25000, 20000)
				return fmt.Sprintf("定額資金前渡制を採用しており、小口現金の使用額 %s について小切手を振り出して補給した。", formatYen(b)),
					side(je("小口現金", b)), side(je("当座預金", b))
			},
		},
		{
			prefix: "ca-other-check", topic: "cash", diff: 2, n: 3,
			expl: "他人 (得意先) 振出しの小切手は通貨代用証券であり、簿記上は現金として扱う。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 55000, 95000, 75000)
				return fmt.Sprintf("売掛金 %s の回収として、得意先振出しの小切手を受け取った。", formatYen(b)),
					side(je("現金", b)), side(je("売掛金", b))
			},
		},
		{
			prefix: "ca-own-check", topic: "cash", diff: 1, n: 3,
			expl: "小切手の振出しは当座預金の減少として処理する (振出時点で貸方計上)。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 65000, 110000, 85000)
				return fmt.Sprintf("買掛金 %s の支払いのため、小切手を振り出した。", formatYen(b)),
					side(je("買掛金", b)), side(je("当座預金", b))
			},
		},
	}
}
