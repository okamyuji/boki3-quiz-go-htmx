package domain

import "time"

// User は永続化される利用者表現で、パスワードハッシュは raw bytes を保持する。
type User struct {
	ID                int64
	Username          string
	PasswordHash      []byte
	PasswordSalt      []byte
	PasswordParams    string
	PasswordUpdatedAt time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
