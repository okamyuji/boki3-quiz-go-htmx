package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

// UserRepo は users テーブルへのアクセスを提供する。
type UserRepo struct {
	db *sql.DB
}

// NewUserRepo は UserRepo を生成する。
func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

var _ port.UserRepository = (*UserRepo)(nil)

const insertUserSQL = `INSERT INTO users(
  username, password_hash, password_salt, password_params, password_updated_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`

const selectUserBaseSQL = `SELECT id, username, password_hash, password_salt, password_params,
  password_updated_at, created_at, updated_at FROM users`

// Create は users へ 1 件追加する。username の重複は domain.ErrAlreadyExists を返す。
func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	now := u.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if u.UpdatedAt.IsZero() {
		u.UpdatedAt = now
	}
	if u.PasswordUpdatedAt.IsZero() {
		u.PasswordUpdatedAt = now
	}
	res, err := r.db.ExecContext(ctx, insertUserSQL,
		u.Username, u.PasswordHash, u.PasswordSalt, u.PasswordParams,
		u.PasswordUpdatedAt.Unix(), now.Unix(), u.UpdatedAt.Unix())
	if err != nil {
		if isUniqueConstraint(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("user create: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("user create last id: %w", err)
	}
	u.ID = id
	u.CreatedAt = now
	return nil
}

// FindByUsername は username (大小無視) で 1 件返す。見つからない場合 domain.ErrNotFound。
func (r *UserRepo) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, selectUserBaseSQL+" WHERE username = ? COLLATE NOCASE", username)
	return scanUser(row)
}

// FindByID は id で 1 件返す。
func (r *UserRepo) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, selectUserBaseSQL+" WHERE id = ?", id)
	return scanUser(row)
}

// UpdatePassword は password_hash/salt/params/password_updated_at/updated_at を更新する。
func (r *UserRepo) UpdatePassword(ctx context.Context, id int64, hash, salt []byte, params string, at time.Time) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET password_hash=?, password_salt=?, password_params=?, password_updated_at=?, updated_at=? WHERE id=?`,
		hash, salt, params, at.Unix(), at.Unix(), id)
	if err != nil {
		return fmt.Errorf("user update password: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("user update rows: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanUser(row *sql.Row) (*domain.User, error) {
	var u domain.User
	var pwUpdated, created, updated int64
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.PasswordSalt, &u.PasswordParams,
		&pwUpdated, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("user scan: %w", err)
	}
	u.PasswordUpdatedAt = time.Unix(pwUpdated, 0).UTC()
	u.CreatedAt = time.Unix(created, 0).UTC()
	u.UpdatedAt = time.Unix(updated, 0).UTC()
	return &u, nil
}

// isUniqueConstraint は UNIQUE 制約違反かを判定する。
// modernc.org/sqlite はメッセージに "UNIQUE constraint failed" を含めるため文字列で判定する。
func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed") ||
		strings.Contains(err.Error(), "constraint failed: UNIQUE")
}
