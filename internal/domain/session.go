package domain

import "time"

// Session は Web UI 用 Cookie セッションを表す。CSRFToken はダブルサブミット用。
type Session struct {
	ID         string
	UserID     int64
	CSRFToken  string
	UserAgent  string
	IP         string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastSeenAt time.Time
}

// IsExpired は now との比較で有効期限切れか判定する。
func (s Session) IsExpired(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}
