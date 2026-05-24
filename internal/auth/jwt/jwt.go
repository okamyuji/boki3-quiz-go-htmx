// Package jwt は HS256 JWT を標準ライブラリのみで自前実装する。
//
// このパッケージは外部ライブラリに依存しない。理由は本プロジェクトの方針
// (標準ライブラリ + 最小限の純 Go 依存) と、JWT 仕様の脆弱性 (alg=none 等) を
// 自前検証で排除するためである。
package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

// HS256Signer は HS256 (HMAC-SHA256) JWT を発行/検証する。
type HS256Signer struct {
	secret []byte
}

// NewHS256 は HS256Signer を生成する。secret は最低 32 バイトを推奨。
func NewHS256(secret []byte) (*HS256Signer, error) {
	if len(secret) < 32 {
		return nil, errors.New("jwt secret too short (need >= 32 bytes)")
	}
	return &HS256Signer{secret: secret}, nil
}

var _ port.JWTSigner = (*HS256Signer)(nil)

type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type payload struct {
	Sub int64  `json:"sub"`
	Iss string `json:"iss"`
	Aud string `json:"aud"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
	JTI string `json:"jti"`
}

// Sign は claims から HS256 JWT を生成する。
func (s *HS256Signer) Sign(claims domain.JWTClaims) (string, error) {
	hdr, err := json.Marshal(header{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		return "", fmt.Errorf("jwt header marshal: %w", err)
	}
	pl, err := json.Marshal(payload{
		Sub: claims.Subject,
		Iss: claims.Issuer,
		Aud: claims.Audience,
		Iat: claims.IssuedAt.Unix(),
		Exp: claims.ExpiresAt.Unix(),
		JTI: claims.JTI,
	})
	if err != nil {
		return "", fmt.Errorf("jwt payload marshal: %w", err)
	}
	signingInput := base64URL(hdr) + "." + base64URL(pl)
	mac := hmac.New(sha256.New, s.secret)
	if _, err := mac.Write([]byte(signingInput)); err != nil {
		return "", fmt.Errorf("jwt hmac: %w", err)
	}
	sig := mac.Sum(nil)
	return signingInput + "." + base64URL(sig), nil
}

// Parse は token を検証し claims を返す。
//
// alg が HS256 でないトークンや、署名が一致しないトークン、exp が過去のトークンは拒否する。
// alg=none や alg=RS256 の confusion 攻撃を排除するため、Header.Alg を明示的に "HS256" 限定する。
func (s *HS256Signer) Parse(token string) (domain.JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return domain.JWTClaims{}, errors.New("jwt: token must have 3 segments")
	}
	rawHeader, err := decodeBase64URL(parts[0])
	if err != nil {
		return domain.JWTClaims{}, fmt.Errorf("jwt header decode: %w", err)
	}
	var hdr header
	if err := json.Unmarshal(rawHeader, &hdr); err != nil {
		return domain.JWTClaims{}, fmt.Errorf("jwt header unmarshal: %w", err)
	}
	if hdr.Alg != "HS256" {
		return domain.JWTClaims{}, fmt.Errorf("jwt: unexpected alg %q", hdr.Alg)
	}
	if hdr.Typ != "" && hdr.Typ != "JWT" {
		return domain.JWTClaims{}, fmt.Errorf("jwt: unexpected typ %q", hdr.Typ)
	}

	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, s.secret)
	if _, err := mac.Write([]byte(signingInput)); err != nil {
		return domain.JWTClaims{}, fmt.Errorf("jwt verify hmac: %w", err)
	}
	expected := mac.Sum(nil)
	sig, err := decodeBase64URL(parts[2])
	if err != nil {
		return domain.JWTClaims{}, fmt.Errorf("jwt sig decode: %w", err)
	}
	if !hmac.Equal(sig, expected) {
		return domain.JWTClaims{}, errors.New("jwt: signature mismatch")
	}

	rawPayload, err := decodeBase64URL(parts[1])
	if err != nil {
		return domain.JWTClaims{}, fmt.Errorf("jwt payload decode: %w", err)
	}
	var p payload
	if err := json.Unmarshal(rawPayload, &p); err != nil {
		return domain.JWTClaims{}, fmt.Errorf("jwt payload unmarshal: %w", err)
	}

	c := domain.JWTClaims{
		Subject:   p.Sub,
		Issuer:    p.Iss,
		Audience:  p.Aud,
		IssuedAt:  time.Unix(p.Iat, 0).UTC(),
		ExpiresAt: time.Unix(p.Exp, 0).UTC(),
		JTI:       p.JTI,
	}
	if !time.Now().UTC().Before(c.ExpiresAt) {
		return domain.JWTClaims{}, errors.New("jwt: token expired")
	}
	return c, nil
}

func base64URL(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeBase64URL(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
