package domain

// Topic は日商簿記 3 級の論点を 1 つ表す。
type Topic struct {
	ID   int64
	Code string
	Name string
	Ord  int
}

// topics は固定の 15 論点。出題区分表 2022 年度版 (2026 年度試験適用) に準拠する。
var topics = []Topic{
	{ID: 1, Code: "basics", Name: "簿記の基本原理", Ord: 1},
	{ID: 2, Code: "books_slips", Name: "帳簿・伝票・証ひょう", Ord: 2},
	{ID: 3, Code: "cash", Name: "現金預金", Ord: 3},
	{ID: 4, Code: "ar_ap", Name: "売掛金と買掛金", Ord: 4},
	{ID: 5, Code: "other_claims", Name: "その他の債権と債務", Ord: 5},
	{ID: 6, Code: "bills", Name: "手形・電子記録債権/債務", Ord: 6},
	{ID: 7, Code: "credit_ar", Name: "クレジット売掛金", Ord: 7},
	{ID: 8, Code: "allowance", Name: "貸倒引当金 (実績法)", Ord: 8},
	{ID: 9, Code: "merchandise", Name: "商品売買 (3 分法)", Ord: 9},
	{ID: 10, Code: "fixed_assets", Name: "有形固定資産", Ord: 10},
	{ID: 11, Code: "income_expense", Name: "収益と費用", Ord: 11},
	{ID: 12, Code: "taxes", Name: "税金", Ord: 12},
	{ID: 13, Code: "closing", Name: "決算整理", Ord: 13},
	{ID: 14, Code: "statements", Name: "試算表/精算表/財務諸表", Ord: 14},
	{ID: 15, Code: "corp_equity", Name: "株式会社会計", Ord: 15},
}

// Topics は 15 論点のコピーを順序付きで返す。
func Topics() []Topic {
	out := make([]Topic, len(topics))
	copy(out, topics)
	return out
}

// TopicByCode は code に対応する論点を返す。見つからないとき ok=false。
func TopicByCode(code string) (Topic, bool) {
	for _, t := range topics {
		if t.Code == code {
			return t, true
		}
	}
	return Topic{}, false
}
