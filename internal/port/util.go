// Package port は repository / service / transport 層で共有される interface を定義する。
package port

import (
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// IDGenerator はトークンや UUID を生成する。CSPRNG を利用する想定。
type IDGenerator interface {
	NewToken(byteLen int) (string, error)
	NewUUID() (string, error)
}

// PasswordHasher はパスワードのハッシュ生成と検証を行う。
// 実装は scrypt + per-user salt + params 文字列 (例: "scrypt$N=...$r=...$p=...$keyLen=...") を使う。
type PasswordHasher interface {
	Hash(plain string) (hash, salt []byte, params string, err error)
	Verify(plain string, hash, salt []byte, params string) (bool, error)
}

// JWTSigner は HS256 JWT の発行と検証を行う。
type JWTSigner interface {
	Sign(claims domain.JWTClaims) (string, error)
	Parse(token string) (domain.JWTClaims, error)
}

// RateLimiter は鍵単位で許容判定を行う。実装ごとに方式 (sliding/fixed/token-bucket) を持つ。
type RateLimiter interface {
	Allow(key string) bool
}
