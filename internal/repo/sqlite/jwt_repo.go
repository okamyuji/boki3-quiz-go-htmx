package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

// JWTRevocationRepo は jwt_revocations テーブルへのアクセスを提供する。
type JWTRevocationRepo struct {
	db *sql.DB
}

// NewJWTRevocationRepo は JWTRevocationRepo を生成する。
func NewJWTRevocationRepo(db *sql.DB) *JWTRevocationRepo { return &JWTRevocationRepo{db: db} }

var _ port.JWTRevocationRepository = (*JWTRevocationRepo)(nil)

// Revoke は JTI を失効リストへ追加する。
// 既に存在する場合は (一意制約に関わらず) 静かに上書きする。
func (r *JWTRevocationRepo) Revoke(ctx context.Context, jti string, userID int64, expiresAt time.Time) error {
	now := time.Now().UTC().Unix()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO jwt_revocations(jti, user_id, revoked_at, expires_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(jti) DO UPDATE SET revoked_at = excluded.revoked_at, expires_at = excluded.expires_at`,
		jti, userID, now, expiresAt.Unix())
	if err != nil {
		return fmt.Errorf("jwt revoke: %w", err)
	}
	return nil
}

// IsRevoked は jti が失効済みかを返す。
func (r *JWTRevocationRepo) IsRevoked(ctx context.Context, jti string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM jwt_revocations WHERE jti = ?`, jti).Scan(&one)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, fmt.Errorf("jwt is revoked: %w", err)
}

// RevokeAllForUser は当該ユーザの全 JWT を失効させる。expires_at は now + 1 日 (上限) とし PurgeExpired で清掃される。
func (r *JWTRevocationRepo) RevokeAllForUser(ctx context.Context, userID int64, now time.Time) error {
	// パスワード変更時の全 JWT 無効化マーカ。JTI に "*" を予約 (rfc7519 の jti は文字列なら何でもよい)。
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO jwt_revocations(jti, user_id, revoked_at, expires_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(jti) DO UPDATE SET revoked_at = excluded.revoked_at, expires_at = excluded.expires_at`,
		fmt.Sprintf("user-all-%d-%d", userID, now.UnixNano()), userID, now.Unix(), now.Add(24*time.Hour).Unix())
	if err != nil {
		return fmt.Errorf("jwt revoke all: %w", err)
	}
	return nil
}

// PurgeExpired は expires_at <= now の失効レコードを削除する。
func (r *JWTRevocationRepo) PurgeExpired(ctx context.Context, now time.Time) (int, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM jwt_revocations WHERE expires_at <= ?`, now.Unix())
	if err != nil {
		return 0, fmt.Errorf("jwt purge: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("jwt purge rows: %w", err)
	}
	return int(n), nil
}
