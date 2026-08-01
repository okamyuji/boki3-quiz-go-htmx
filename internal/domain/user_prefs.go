package domain

import "time"

// UserPrefs はユーザーごとの学習画面設定 (セット/モード) を表す。
type UserPrefs struct {
	UserID    int64
	QuizSet   string
	QuizMode  QuizMode
	UpdatedAt time.Time
}
