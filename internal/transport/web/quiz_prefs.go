package web

import (
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// quizPrefsInput は resolveQuizPrefs への入力。
type quizPrefsInput struct {
	QuerySet  string
	QueryMode string
	Stored    *domain.UserPrefs
	SetExists func(string) bool
}

// resolveQuizPrefs は学習画面のセット/モードを決定する。
// 優先順位: 検証済みクエリパラメータ > 保存済み設定 > 初期値。
// save はクエリで明示的に切り替えられ、保存値と異なる場合に true。
func resolveQuizPrefs(in quizPrefsInput) (setCode string, mode domain.QuizMode, save bool) {
	// 各ソースを検証し、無効な値は「無指定」として扱う。
	querySet := in.QuerySet
	if querySet != "" && !in.SetExists(querySet) {
		querySet = ""
	}
	queryMode := domain.QuizMode(in.QueryMode)
	if queryMode != "" && !queryMode.IsValid() {
		queryMode = ""
	}
	var storedSet string
	var storedMode domain.QuizMode
	if in.Stored != nil {
		if in.SetExists(in.Stored.QuizSet) {
			storedSet = in.Stored.QuizSet
		}
		if in.Stored.QuizMode.IsValid() {
			storedMode = in.Stored.QuizMode
		}
	}

	setCode = firstNonEmpty(querySet, storedSet, domain.SetCodeCore)
	mode = domain.QuizMode(firstNonEmpty(string(queryMode), string(storedMode), string(domain.QuizModeSRS)))

	explicit := querySet != "" || queryMode != ""
	changed := in.Stored == nil || in.Stored.QuizSet != setCode || in.Stored.QuizMode != mode
	save = explicit && changed
	return setCode, mode, save
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
