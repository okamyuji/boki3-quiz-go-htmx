package accounts_test

import (
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/accounts"
)

func TestNormalize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		// 正常系: 標準名はそのまま
		{"標準名そのまま", "売上", "売上"},
		{"複合科目そのまま", "備品減価償却累計額", "備品減価償却累計額"},
		// 別名 → 標準名
		{"送り仮名: 売り上げ", "売り上げ", "売上"},
		{"送り仮名: 仕入れ", "仕入れ", "仕入"},
		{"送り仮名: 繰り越し商品", "繰り越し商品", "繰越商品"},
		{"慣用別名: 未収金", "未収金", "未収入金"},
		{"送り仮名: 預かり金", "預かり金", "預り金"},
		// 空白の除去 (半角・全角・タブ・別名との複合)
		{"半角空白", "現 金", "現金"},
		{"全角空白", "現　金", "現金"},
		{"タブ", "現\t金", "現金"},
		{"前後空白", " 売上 ", "売上"},
		{"空白+別名", "売り 上げ", "売上"},
		// 境界値・エッジケース
		{"空文字", "", ""},
		{"空白のみ", " 　\t", ""},
		{"未知の表記はそのまま", "売上高", "売上高"},
		{"かな書きは救済しない", "うりあげ", "うりあげ"},
		{"別科目は変換しない", "買掛金", "買掛金"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := accounts.Normalize(c.in); got != c.want {
				t.Fatalf("Normalize(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// 別名表の値はすべて標準科目リストに含まれる (存在しない科目への写像を防ぐ)。
func TestAliasesMapOntoStandard(t *testing.T) {
	t.Parallel()
	std := map[string]bool{}
	for _, a := range accounts.Standard {
		std[a] = true
	}
	// 別名表そのものは非公開なので、代表的な別名の写像先が標準リストに
	// あることを Normalize 経由で確認する。
	for _, alias := range []string{
		"売り上げ", "仕入れ", "繰り越し商品", "未収金", "預かり金",
		"貸倒れ引当金繰入れ", "償却債権取り立て益", "前受け地代",
	} {
		got := accounts.Normalize(alias)
		if got == alias {
			t.Fatalf("alias %q not mapped", alias)
		}
		if !std[got] {
			t.Fatalf("alias %q maps to %q which is not a standard account", alias, got)
		}
	}
}

// Standard に重複がない。
func TestStandardHasNoDuplicates(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, a := range accounts.Standard {
		if seen[a] {
			t.Fatalf("duplicate standard account %q", a)
		}
		seen[a] = true
	}
}
