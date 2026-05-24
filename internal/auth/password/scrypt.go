// Package password は scrypt によるパスワードハッシュ生成と検証を提供する。
//
// パラメータ文字列は "scrypt$N=<int>$r=<int>$p=<int>$keyLen=<int>" 形式で保存する。
// 既定値は OWASP 推奨に従う (N=32768, r=8, p=1, keyLen=32, salt=16 bytes)。
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/scrypt"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

// Params は scrypt パラメータの組。
type Params struct {
	N       int // CPU/メモリコスト (2^N 形式ではなく 32768 等の値)
	R       int
	P       int
	KeyLen  int
	SaltLen int
}

// DefaultParams は OWASP 推奨。
func DefaultParams() Params {
	return Params{N: 32768, R: 8, P: 1, KeyLen: 32, SaltLen: 16}
}

// ScryptHasher は scrypt を用いた PasswordHasher 実装。
type ScryptHasher struct {
	params Params
}

// New は ScryptHasher を生成する。
func New(p Params) *ScryptHasher { return &ScryptHasher{params: p} }

// Default は OWASP 推奨パラメータの ScryptHasher を返す。
func Default() *ScryptHasher { return New(DefaultParams()) }

var _ port.PasswordHasher = (*ScryptHasher)(nil)

// Hash は plain から salt と key を生成し、ハッシュ・salt・params 文字列を返す。
func (h *ScryptHasher) Hash(plain string) (hash, salt []byte, params string, err error) {
	if plain == "" {
		return nil, nil, "", errors.New("password is empty")
	}
	salt = make([]byte, h.params.SaltLen)
	if _, err = rand.Read(salt); err != nil {
		return nil, nil, "", fmt.Errorf("scrypt salt: %w", err)
	}
	hash, err = scrypt.Key([]byte(plain), salt, h.params.N, h.params.R, h.params.P, h.params.KeyLen)
	if err != nil {
		return nil, nil, "", fmt.Errorf("scrypt key: %w", err)
	}
	return hash, salt, encodeParams(h.params), nil
}

// Verify は params をパースし plain を hash と比較する (一定時間比較)。
func (h *ScryptHasher) Verify(plain string, hash, salt []byte, params string) (bool, error) {
	p, err := decodeParams(params)
	if err != nil {
		return false, fmt.Errorf("verify params: %w", err)
	}
	candidate, err := scrypt.Key([]byte(plain), salt, p.N, p.R, p.P, p.KeyLen)
	if err != nil {
		return false, fmt.Errorf("verify key: %w", err)
	}
	return subtle.ConstantTimeCompare(candidate, hash) == 1, nil
}

func encodeParams(p Params) string {
	var sb strings.Builder
	sb.WriteString("scrypt$N=")
	sb.WriteString(strconv.Itoa(p.N))
	sb.WriteString("$r=")
	sb.WriteString(strconv.Itoa(p.R))
	sb.WriteString("$p=")
	sb.WriteString(strconv.Itoa(p.P))
	sb.WriteString("$keyLen=")
	sb.WriteString(strconv.Itoa(p.KeyLen))
	return sb.String()
}

func decodeParams(s string) (Params, error) {
	if !strings.HasPrefix(s, "scrypt$") {
		return Params{}, errors.New("not scrypt params")
	}
	tail := strings.TrimPrefix(s, "scrypt$")
	parts := strings.Split(tail, "$")
	p := Params{}
	for _, kv := range parts {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return Params{}, fmt.Errorf("bad pair %q", kv)
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return Params{}, fmt.Errorf("bad value for %s: %w", k, err)
		}
		switch k {
		case "N":
			p.N = n
		case "r":
			p.R = n
		case "p":
			p.P = n
		case "keyLen":
			p.KeyLen = n
		default:
			return Params{}, fmt.Errorf("unknown key %q", k)
		}
	}
	if p.N == 0 || p.R == 0 || p.P == 0 || p.KeyLen == 0 {
		return Params{}, errors.New("missing scrypt parameters")
	}
	return p, nil
}
