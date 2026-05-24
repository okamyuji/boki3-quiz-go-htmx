// Package service はアプリケーションのユースケース (組み立て) を提供する。
//
// すべての依存は interface (internal/port) を経由して受け取り、handler/transport 層は
// このパッケージの公開関数のみを呼ぶ。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/clock"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

// AuthConfig は AuthService の挙動パラメータ。
type AuthConfig struct {
	SessionTTL      time.Duration // セッション cookie 寿命
	SessionTokenLen int           // セッション ID のバイト長 (16 推奨)
	CSRFTokenLen    int           // CSRF トークンのバイト長 (16 推奨)
	MinPasswordLen  int           // 最小パスワード長 (12 推奨)
}

// DefaultAuthConfig は本番既定値を返す。
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		SessionTTL:      14 * 24 * time.Hour,
		SessionTokenLen: 32,
		CSRFTokenLen:    32,
		MinPasswordLen:  12,
	}
}

// AuthService は port.AuthService の実装。
type AuthService struct {
	users    port.UserRepository
	sessions port.SessionRepository
	jwts     port.JWTRevocationRepository
	hasher   port.PasswordHasher
	idgen    port.IDGenerator
	clock    clock.Clock
	cfg      AuthConfig
}

// NewAuthService は AuthService を生成する。
func NewAuthService(
	users port.UserRepository,
	sessions port.SessionRepository,
	jwts port.JWTRevocationRepository,
	hasher port.PasswordHasher,
	idgen port.IDGenerator,
	clk clock.Clock,
	cfg AuthConfig,
) *AuthService {
	if clk == nil {
		clk = clock.System{}
	}
	return &AuthService{users: users, sessions: sessions, jwts: jwts, hasher: hasher, idgen: idgen, clock: clk, cfg: cfg}
}

var _ port.AuthService = (*AuthService)(nil)

// Register は新規ユーザを登録する。重複は domain.ErrAlreadyExists。
func (a *AuthService) Register(ctx context.Context, username, plain string) (*domain.User, error) {
	if err := validateUsername(username); err != nil {
		return nil, err
	}
	if err := a.validatePassword(plain); err != nil {
		return nil, err
	}
	hash, salt, params, err := a.hasher.Hash(plain)
	if err != nil {
		return nil, fmt.Errorf("auth register hash: %w", err)
	}
	now := a.clock.Now()
	u := &domain.User{
		Username:          username,
		PasswordHash:      hash,
		PasswordSalt:      salt,
		PasswordParams:    params,
		PasswordUpdatedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := a.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Login はユーザ名/パスワード検証後、新規セッションを発行する。
// レートリミットは呼び出し側 (handler) で行う。
//
// 「ユーザ不在」と「パスワード不一致」を timing で区別させないために、
// ユーザ不在の場合もダミー Verify を 1 回走らせて scrypt と同等の CPU 時間を消費する。
func (a *AuthService) Login(ctx context.Context, username, plain, userAgent, ip string) (*domain.Session, error) {
	u, err := a.users.FindByUsername(ctx, username)
	if err != nil {
		// timing 対策: 失敗時もハッシュ計算を 1 回走らせる (結果は破棄)。
		_, _ = a.hasher.Verify(plain, dummyTimingHash, dummyTimingSalt, dummyTimingParams)
		return nil, domain.ErrUnauthorized
	}
	ok, err := a.hasher.Verify(plain, u.PasswordHash, u.PasswordSalt, u.PasswordParams)
	if err != nil {
		return nil, fmt.Errorf("auth login verify: %w", err)
	}
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	sid, err := a.idgen.NewToken(a.cfg.SessionTokenLen)
	if err != nil {
		return nil, fmt.Errorf("auth login sid: %w", err)
	}
	csrf, err := a.idgen.NewToken(a.cfg.CSRFTokenLen)
	if err != nil {
		return nil, fmt.Errorf("auth login csrf: %w", err)
	}
	now := a.clock.Now()
	s := &domain.Session{
		ID:         sid,
		UserID:     u.ID,
		CSRFToken:  csrf,
		UserAgent:  truncate(userAgent, 1024),
		IP:         truncate(ip, 64),
		CreatedAt:  now,
		ExpiresAt:  now.Add(a.cfg.SessionTTL),
		LastSeenAt: now,
	}
	if err := a.sessions.Create(ctx, s); err != nil {
		return nil, fmt.Errorf("auth login session: %w", err)
	}
	return s, nil
}

// Logout はセッションを削除する。
func (a *AuthService) Logout(ctx context.Context, sessionID string) error {
	return a.sessions.Delete(ctx, sessionID)
}

// ChangePassword は旧パスワード検証後、新パスワードへ更新し、他セッションを破棄、
// JWT 全失効マーカを置く。
func (a *AuthService) ChangePassword(ctx context.Context, userID int64, currentPlain, newPlain, keepSessionID string) error {
	if err := a.validatePassword(newPlain); err != nil {
		return err
	}
	u, err := a.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	ok, err := a.hasher.Verify(currentPlain, u.PasswordHash, u.PasswordSalt, u.PasswordParams)
	if err != nil {
		return fmt.Errorf("auth change pw verify: %w", err)
	}
	if !ok {
		return domain.ErrPasswordMismatch
	}
	hash, salt, params, err := a.hasher.Hash(newPlain)
	if err != nil {
		return fmt.Errorf("auth change pw hash: %w", err)
	}
	now := a.clock.Now()
	if err := a.users.UpdatePassword(ctx, userID, hash, salt, params, now); err != nil {
		return fmt.Errorf("auth change pw update: %w", err)
	}
	if keepSessionID != "" {
		if err := a.sessions.DeleteAllForUserExcept(ctx, userID, keepSessionID); err != nil {
			return fmt.Errorf("auth change pw sessions: %w", err)
		}
	} else {
		if err := a.sessions.DeleteAllForUser(ctx, userID); err != nil {
			return fmt.Errorf("auth change pw sessions: %w", err)
		}
	}
	if err := a.jwts.RevokeAllForUser(ctx, userID, now); err != nil {
		return fmt.Errorf("auth change pw jwt: %w", err)
	}
	return nil
}

// SessionByID はセッションとそのユーザを返す。
// セッション期限切れの場合 domain.ErrSessionExpired を返す。
func (a *AuthService) SessionByID(ctx context.Context, sessionID string) (*domain.Session, *domain.User, error) {
	s, err := a.sessions.FindByID(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	if s.IsExpired(a.clock.Now()) {
		_ = a.sessions.Delete(ctx, sessionID)
		return nil, nil, domain.ErrSessionExpired
	}
	u, err := a.users.FindByID(ctx, s.UserID)
	if err != nil {
		return nil, nil, err
	}
	if err := a.sessions.Touch(ctx, sessionID, a.clock.Now()); err != nil {
		// touch 失敗は致命でない (log は呼出側)
		_ = err
	}
	return s, u, nil
}

// validateUsername は username の形式を検証する。3..32 文字、英数字とアンダースコアのみ。
func validateUsername(u string) error {
	if u == "" {
		return domain.ErrUsernameInvalid
	}
	if utf8.RuneCountInString(u) < 3 || utf8.RuneCountInString(u) > 32 {
		return domain.ErrUsernameInvalid
	}
	for _, r := range u {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return domain.ErrUsernameInvalid
		}
	}
	return nil
}

func (a *AuthService) validatePassword(p string) error {
	if utf8.RuneCountInString(p) < a.cfg.MinPasswordLen {
		return domain.ErrPasswordTooWeak
	}
	hasLower := false
	hasUpper := false
	hasDigit := false
	hasSym := false
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		case strings.ContainsRune("!@#$%^&*()-_=+[]{};:,.<>/?\\|`~'\"", r):
			hasSym = true
		}
	}
	count := 0
	for _, b := range []bool{hasLower, hasUpper, hasDigit, hasSym} {
		if b {
			count++
		}
	}
	if count < 3 {
		return domain.ErrPasswordTooWeak
	}
	return nil
}

// truncate は UTF-8 を壊さず先頭から max ルーンで切り詰めて返す。
// dummyTiming* は timing-attack 緩和用のダミー値。実 hash と同じ shape (params + 同サイズ salt/hash)。
var (
	dummyTimingHash   = make([]byte, 32)
	dummyTimingSalt   = make([]byte, 16)
	dummyTimingParams = "scrypt$N=32768$r=8$p=1$keyLen=32"
)

// truncate は UTF-8 を壊さず先頭から maxRunes ルーンで切り詰めて返す。
func truncate(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes])
}
