package domain

import "time"

// JWTClaims は HS256 JWT の本アプリ向けクレーム。
// 標準クレーム名は RFC 7519 (sub/iat/exp/iss/aud/jti) に従う。
type JWTClaims struct {
	Subject   int64     // sub: user_id
	Issuer    string    // iss: "boki3-quiz"
	Audience  string    // aud: "api"
	IssuedAt  time.Time // iat
	ExpiresAt time.Time // exp
	JTI       string    // jti
}
