package seed

import (
	"fmt"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// receivablePatterns は売掛金・買掛金・貸倒れの文型群。
func receivablePatterns() []pattern {
	return []pattern{
		{
			prefix: "rc-collect-bank", topic: "receivable", diff: 1, n: 3,
			expl: "売掛金の回収により資産が振り替わる。借方に普通預金、貸方に売掛金。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 85000, 140000, 230000)
				return fmt.Sprintf("売掛金 %s が普通預金口座に振り込まれた。", formatYen(b)),
					side(je("普通預金", b)), side(je("売掛金", b))
			},
		},
		{
			prefix: "rc-pay-cash", topic: "receivable", diff: 1, n: 3,
			expl: "買掛金の支払いにより負債が減少する。借方に買掛金、貸方に現金。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 45000, 90000, 65000)
				return fmt.Sprintf("買掛金 %s を現金で支払った。", formatYen(b)),
					side(je("買掛金", b)), side(je("現金", b))
			},
		},
		{
			prefix: "rc-allowance", topic: "receivable", diff: 2, n: 3,
			expl: "差額補充法では、設定額 (残高 × 見積率) と決算整理前残高との差額だけを貸倒引当金繰入とする。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 300000, 500000, 400000)
				r := pick(i, 2000, 3000, 4000)
				add := b*2/100 - r
				return fmt.Sprintf("決算整理: 売掛金残高 %s に対して 2%% の貸倒引当金を差額補充法により設定する。決算整理前の貸倒引当金残高は %s である。",
						formatYen(b), formatYen(r)),
					side(je("貸倒引当金繰入", add)), side(je("貸倒引当金", add))
			},
		},
		{
			prefix: "rc-writeoff-full", topic: "receivable", diff: 2, n: 3,
			expl: "前期以前に発生した売掛金の貸倒れは、設定済みの貸倒引当金を取り崩して充当する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 30000, 60000, 45000)
				return fmt.Sprintf("前期に発生した売掛金 %s が得意先の倒産により貸し倒れた。貸倒引当金の残高は貸倒額を上回っている。", formatYen(b)),
					side(je("貸倒引当金", b)), side(je("売掛金", b))
			},
		},
		{
			prefix: "rc-writeoff-part", topic: "receivable", diff: 2, n: 3,
			expl: "貸倒額が引当金残高を超える場合、超過分は貸倒損失 (費用) として処理する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 50000, 80000, 100000)
				r := b * 3 / 5
				return fmt.Sprintf("前期に発生した売掛金 %s が貸し倒れた。貸倒引当金の残高は %s である。",
						formatYen(b), formatYen(r)),
					side(je("貸倒引当金", r), je("貸倒損失", b-r)), side(je("売掛金", b))
			},
		},
		{
			prefix: "rc-writeoff-current", topic: "receivable", diff: 2, n: 3,
			expl: "当期に発生した売掛金の貸倒れには引当金を充当せず、全額を貸倒損失とする。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 25000, 40000, 32000)
				return fmt.Sprintf("当期に発生した売掛金 %s が得意先の倒産により貸し倒れた。", formatYen(b)),
					side(je("貸倒損失", b)), side(je("売掛金", b))
			},
		},
		{
			prefix: "rc-recovered", topic: "receivable", diff: 2, n: 3,
			expl: "前期以前に貸倒処理した債権を回収したときは、償却債権取立益 (収益) を計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 15000, 30000, 22000)
				return fmt.Sprintf("前期に貸倒れとして処理した売掛金のうち %s を現金で回収した。", formatYen(b)),
					side(je("現金", b)), side(je("償却債権取立益", b))
			},
		},
		{
			prefix: "rc-cc-collect", topic: "receivable", diff: 2, n: 3,
			expl: "信販会社からの入金により、クレジット売掛金を減少させる。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 48000, 96000, 72000)
				return fmt.Sprintf("信販会社からクレジット売掛金 %s が普通預金口座に入金された。", formatYen(b)),
					side(je("普通預金", b)), side(je("クレジット売掛金", b))
			},
		},
	}
}
