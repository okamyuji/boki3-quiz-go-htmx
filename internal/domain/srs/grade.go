// Package srs は SM-2 風の間隔反復アルゴリズムを提供する。
package srs

// Grade は SM-2 の品質スコア (0..5)。
type Grade int

// Grade の列挙値。0 が最悪、5 が完璧 (SM-2 原典に準ずる)。
const (
	GradeWorst   Grade = 0
	GradeFail    Grade = 2
	GradeSlow    Grade = 3
	GradeGood    Grade = 4
	GradePerfect Grade = 5
)

// GradeFromResult は正誤と解答時間 (ms) から Grade を導く。
//   - 不正解 -> 0
//   - 正解 5s 未満 -> 5
//   - 正解 5s 以上 15s 未満 -> 4
//   - 正解 15s 以上 -> 3
func GradeFromResult(correct bool, durationMs int) Grade {
	if !correct {
		return GradeWorst
	}
	switch {
	case durationMs < 5000:
		return GradePerfect
	case durationMs < 15000:
		return GradeGood
	default:
		return GradeSlow
	}
}
