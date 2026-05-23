package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

// SessionRepo は sessions テーブルへのアクセスを提供する。
type SessionRepo struct {
	db *sql.DB
}

// NewSessionRepo は SessionRepo を生成する。
func NewSessionRepo(db *sql.DB) *SessionRepo { return &SessionRepo{db: db} }

var _ port.SessionRepository = (*SessionRepo)(nil)

const insertSessionSQL = `INSERT INTO sessions(
  id, user_id, csrf_token, user_agent, ip, created_at, expires_at, last_seen_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

// Create は新規セッションを追加する。
func (r *SessionRepo) Create(ctx context.Context, s *domain.Session) error {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	if s.LastSeenAt.IsZero() {
		s.LastSeenAt = s.CreatedAt
	}
	_, err := r.db.ExecContext(ctx, insertSessionSQL,
		s.ID, s.UserID, s.CSRFToken, s.UserAgent, s.IP,
		s.CreatedAt.Unix(), s.ExpiresAt.Unix(), s.LastSeenAt.Unix())
	if err != nil {
		if isUniqueConstraint(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("session create: %w", err)
	}
	return nil
}

// FindByID は id でセッションを返す。見つからない場合 domain.ErrNotFound。
func (r *SessionRepo) FindByID(ctx context.Context, id string) (*domain.Session, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, csrf_token, COALESCE(user_agent,''), COALESCE(ip,''),
		 created_at, expires_at, last_seen_at FROM sessions WHERE id = ?`, id)
	var s domain.Session
	var created, expires, lastSeen int64
	if err := row.Scan(&s.ID, &s.UserID, &s.CSRFToken, &s.UserAgent, &s.IP,
		&created, &expires, &lastSeen); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("session find: %w", err)
	}
	s.CreatedAt = time.Unix(created, 0).UTC()
	s.ExpiresAt = time.Unix(expires, 0).UTC()
	s.LastSeenAt = time.Unix(lastSeen, 0).UTC()
	return &s, nil
}

// Touch は last_seen_at を更新する。
func (r *SessionRepo) Touch(ctx context.Context, id string, lastSeen time.Time) error {
	res, err := r.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ? WHERE id = ?`, lastSeen.Unix(), id)
	if err != nil {
		return fmt.Errorf("session touch: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("session touch rows: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete は id でセッションを削除する。存在しない場合は no-op。
func (r *SessionRepo) Delete(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("session delete: %w", err)
	}
	return nil
}

// DeleteAllForUser はユーザのすべてのセッションを削除する。
func (r *SessionRepo) DeleteAllForUser(ctx context.Context, userID int64) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("session delete user: %w", err)
	}
	return nil
}

// DeleteAllForUserExcept は keepID 以外のセッションを削除する (パスワード変更後に他デバイスを破棄するため)。
func (r *SessionRepo) DeleteAllForUserExcept(ctx context.Context, userID int64, keepID string) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = ? AND id != ?`, userID, keepID); err != nil {
		return fmt.Errorf("session delete except: %w", err)
	}
	return nil
}

// PurgeExpired は expires_at <= now のレコードを削除し、削除件数を返す。
func (r *SessionRepo) PurgeExpired(ctx context.Context, now time.Time) (int, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Unix())
	if err != nil {
		return 0, fmt.Errorf("session purge: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("session purge rows: %w", err)
	}
	return int(n), nil
}
