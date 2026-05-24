package service

import (
	"context"
	"fmt"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/clock"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

// APIAuthService は port.APIAuthService の実装。JWT 発行/検証を提供する。
type APIAuthService struct {
	signer port.JWTSigner
	jwts   port.JWTRevocationRepository
	idgen  port.IDGenerator
	clock  clock.Clock
	issuer string
	aud    string
}

// NewAPIAuthService は APIAuthService を生成する。
func NewAPIAuthService(signer port.JWTSigner, jwts port.JWTRevocationRepository,
	idgen port.IDGenerator, clk clock.Clock, issuer, audience string,
) *APIAuthService {
	if clk == nil {
		clk = clock.System{}
	}
	return &APIAuthService{signer: signer, jwts: jwts, idgen: idgen, clock: clk, issuer: issuer, aud: audience}
}

var _ port.APIAuthService = (*APIAuthService)(nil)

// IssueToken は userID 向けの JWT を発行する。
func (a *APIAuthService) IssueToken(_ context.Context, userID int64, ttl time.Duration) (token, jti string, expiresAt time.Time, err error) {
	now := a.clock.Now()
	expiresAt = now.Add(ttl)
	jti, err = a.idgen.NewUUID()
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("api auth jti: %w", err)
	}
	c := domain.JWTClaims{
		Subject:   userID,
		Issuer:    a.issuer,
		Audience:  a.aud,
		IssuedAt:  now,
		ExpiresAt: expiresAt,
		JTI:       jti,
	}
	token, err = a.signer.Sign(c)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("api auth sign: %w", err)
	}
	return token, jti, expiresAt, nil
}

// VerifyToken はトークンを検証し subject (userID) を返す。
// 期限切れ・失効済み・iss/aud ミスマッチは ErrTokenInvalid を返す。
func (a *APIAuthService) VerifyToken(ctx context.Context, token string) (int64, error) {
	c, err := a.signer.Parse(token)
	if err != nil {
		return 0, domain.ErrTokenInvalid
	}
	now := a.clock.Now()
	if !c.ExpiresAt.After(now) {
		return 0, domain.ErrTokenExpired
	}
	if c.Issuer != a.issuer || c.Audience != a.aud {
		return 0, domain.ErrTokenInvalid
	}
	revoked, err := a.jwts.IsRevoked(ctx, c.JTI)
	if err != nil {
		return 0, fmt.Errorf("api auth jwt revoked check: %w", err)
	}
	if revoked {
		return 0, domain.ErrTokenInvalid
	}
	return c.Subject, nil
}

// Revoke は jti を失効リストへ追加する。
func (a *APIAuthService) Revoke(ctx context.Context, jti string, userID int64, expiresAt time.Time) error {
	return a.jwts.Revoke(ctx, jti, userID, expiresAt)
}

// ExtractJTI はトークンをパースし jti と expiresAt を返す。
// 署名は検証するが、失効リスト照合はしない (logout 用)。
func (a *APIAuthService) ExtractJTI(token string) (string, time.Time, error) {
	c, err := a.signer.Parse(token)
	if err != nil {
		return "", time.Time{}, err
	}
	return c.JTI, c.ExpiresAt, nil
}
