package seed

import (
	"fmt"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// adjustmentPatterns は決算整理・決算振替と棚卸 (三分法) の文型群。
func adjustmentPatterns() []pattern {
	return []pattern{
		{
			prefix: "ad-dep-annual", topic: "adjustment", diff: 2, n: 2,
			expl: "定額法の減価償却費 = (取得原価 − 残存価額) / 耐用年数。間接法では備品減価償却累計額を貸方に計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				c := pick(i, 600000, 800000)
				years := pick(i, 5, 8)
				dep := c / years
				return fmt.Sprintf("決算整理: 備品 (取得原価 %s、耐用年数 %d 年、残存価額ゼロ、定額法・間接法) の減価償却を行う。",
						formatYen(c), years),
					side(je("減価償却費", dep)), side(je("備品減価償却累計額", dep))
			},
		},
		{
			prefix: "ad-dep-month", topic: "adjustment", diff: 2, n: 2,
			expl: "期中取得の固定資産は月割で減価償却する。年間償却費 × 使用月数 / 12。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				c := pick(i, 480000, 360000)
				years := pick(i, 8, 6)
				months := pick(i, 6, 4)
				dep := c / years * months / 12
				return fmt.Sprintf("決算整理: 期中に取得した備品 (取得原価 %s、耐用年数 %d 年、残存価額ゼロ、定額法・間接法) について、使用期間 %d か月分の減価償却を月割で行う。",
						formatYen(c), years, months),
					side(je("減価償却費", dep)), side(je("備品減価償却累計額", dep))
			},
		},
		{
			prefix: "ad-prepaid-ins", topic: "adjustment", diff: 2, n: 2,
			expl: "当期に支払った保険料のうち翌期分は、前払保険料 (資産) として費用から控除する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 12000, 18000)
				return fmt.Sprintf("決算整理: 支払済みの保険料のうち %s は翌期分の前払いである。", formatYen(b)),
					side(je("前払保険料", b)), side(je("保険料", b))
			},
		},
		{
			prefix: "ad-accrued-int", topic: "adjustment", diff: 2, n: 2,
			expl: "当期に属する未払いの利息は、支払利息を追加計上し未払利息 (負債) を立てる。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 6000, 9000)
				return fmt.Sprintf("決算整理: 借入金の利息のうち、当期分の未払額 %s を計上する。", formatYen(b)),
					side(je("支払利息", b)), side(je("未払利息", b))
			},
		},
		{
			prefix: "ad-deferred-rent", topic: "adjustment", diff: 2, n: 2,
			expl: "受け取った家賃のうち翌期分は、前受家賃 (負債) として収益から控除する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 30000, 45000)
				return fmt.Sprintf("決算整理: 受取済みの家賃のうち %s は翌期分の前受けである。", formatYen(b)),
					side(je("受取家賃", b)), side(je("前受家賃", b))
			},
		},
		{
			prefix: "ad-accrued-recv-int", topic: "adjustment", diff: 2, n: 2,
			expl: "当期に属する未収の利息は、未収利息 (資産) を立てて受取利息を追加計上する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 5000, 8000)
				return fmt.Sprintf("決算整理: 貸付金の利息のうち、当期分の未収額 %s を計上する。", formatYen(b)),
					side(je("未収利息", b)), side(je("受取利息", b))
			},
		},
		{
			prefix: "ad-stamp-stock", topic: "adjustment", diff: 2, n: 2,
			expl: "期末に未使用の収入印紙は、貯蔵品 (資産) へ振り替えて費用から控除する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 3000, 7000)
				return fmt.Sprintf("決算整理: 租税公課として処理していた収入印紙のうち %s が未使用のまま残っている。", formatYen(b)),
					side(je("貯蔵品", b)), side(je("租税公課", b))
			},
		},
		{
			prefix: "ad-postage-stock", topic: "adjustment", diff: 2, n: 2,
			expl: "期末に未使用の郵便切手は、貯蔵品 (資産) へ振り替えて通信費から控除する。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 2000, 5000)
				return fmt.Sprintf("決算整理: 通信費として処理していた郵便切手のうち %s が未使用のまま残っている。", formatYen(b)),
					side(je("貯蔵品", b)), side(je("通信費", b))
			},
		},
		{
			prefix: "ad-reversal-ins", topic: "adjustment", diff: 2, n: 2,
			expl: "経過勘定は翌期首に再振替仕訳を行い、前払保険料を保険料へ戻す。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 12000, 18000)
				return fmt.Sprintf("翌期首: 前期末に計上した前払保険料 %s について再振替仕訳を行う。", formatYen(b)),
					side(je("保険料", b)), side(je("前払保険料", b))
			},
		},
		{
			prefix: "ad-short-close", topic: "adjustment", diff: 2, n: 2,
			expl: "決算まで原因不明の現金過不足 (借方残高) は、判明分を該当勘定へ、残額を雑損へ振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 5000, 8000)
				x := pick(i, 3000, 6000)
				return fmt.Sprintf("決算整理: 現金過不足勘定の借方残高 %s のうち %s は通信費の記帳漏れと判明したが、残額は原因不明である。",
						formatYen(b), formatYen(x)),
					side(je("通信費", x), je("雑損", b-x)), side(je("現金過不足", b))
			},
		},
		{
			prefix: "ad-over-close", topic: "adjustment", diff: 2, n: 2,
			expl: "決算まで原因不明の現金過不足 (貸方残高) は、雑益へ振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 4000, 6000)
				return fmt.Sprintf("決算整理: 現金過不足勘定の貸方残高 %s は、原因が判明しないため適切な勘定へ振り替える。", formatYen(b)),
					side(je("現金過不足", b)), side(je("雑益", b))
			},
		},
		{
			prefix: "ad-overdraft", topic: "adjustment", diff: 2, n: 2,
			expl: "決算日に当座預金勘定が貸方残高 (引出超過) の場合、当座借越 (負債) へ振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 150000, 220000)
				return fmt.Sprintf("決算整理: 当座預金勘定が %s の貸方残高となっているため、適切な勘定へ振り替える (当座借越勘定を用いること)。", formatYen(b)),
					side(je("当座預金", b)), side(je("当座借越", b))
			},
		},
		{
			prefix: "iv-shift", topic: "inventory", diff: 2, n: 2,
			expl: "三分法の売上原価算定: 期首商品を仕入へ振り替え (仕入/繰越商品)、期末商品を在庫へ戻す (繰越商品/仕入)。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				beg := pick(i, 90000, 120000)
				end := pick(i, 70000, 95000)
				return fmt.Sprintf("決算整理: 期首商品棚卸高は %s、期末商品棚卸高は %s である。売上原価を仕入勘定で算定する (三分法)。",
						formatYen(beg), formatYen(end)),
					side(je("仕入", beg), je("繰越商品", end)),
					side(je("繰越商品", beg), je("仕入", end))
			},
		},
		{
			prefix: "ad-close-sales", topic: "adjustment", diff: 2, n: 2,
			expl: "決算振替: 収益の勘定 (売上) の残高は損益勘定の貸方へ振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 900000, 1500000)
				return fmt.Sprintf("決算振替: 売上勘定の残高 %s を損益勘定へ振り替える。", formatYen(b)),
					side(je("売上", b)), side(je("損益", b))
			},
		},
		{
			prefix: "ad-close-purchase", topic: "adjustment", diff: 2, n: 2,
			expl: "決算振替: 費用の勘定 (仕入 = 売上原価) の残高は損益勘定の借方へ振り替える。",
			build: func(i int) (string, []domain.JournalEntry, []domain.JournalEntry) {
				b := pick(i, 600000, 1000000)
				return fmt.Sprintf("決算振替: 売上原価を算定済みの仕入勘定の残高 %s を損益勘定へ振り替える。", formatYen(b)),
					side(je("損益", b)), side(je("仕入", b))
			},
		},
	}
}
