// Package idgen は CSPRNG ベースのトークン生成と UUIDv4 を提供する。
package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

// CryptoGen は crypto/rand ベースの ID 生成器。
type CryptoGen struct{}

// New は CryptoGen を生成する。
func New() *CryptoGen { return &CryptoGen{} }

var _ port.IDGenerator = (*CryptoGen)(nil)

// NewToken は byteLen バイトの乱数を hex 文字列で返す (長さ 2*byteLen)。
func (CryptoGen) NewToken(byteLen int) (string, error) {
	if byteLen <= 0 {
		return "", errors.New("byteLen must be positive")
	}
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("idgen token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// NewUUID は RFC 4122 v4 UUID を生成して返す。
func (CryptoGen) NewUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("idgen uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
