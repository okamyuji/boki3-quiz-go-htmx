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

// UserPrefsRepo は user_prefs テーブルへのアクセスを提供する。
type UserPrefsRepo struct {
	db *sql.DB
}

// NewUserPrefsRepo は UserPrefsRepo を生成する。
func NewUserPrefsRepo(db *sql.DB) *UserPrefsRepo { return &UserPrefsRepo{db: db} }

var _ port.UserPrefsRepository = (*UserPrefsRepo)(nil)

// Get はユーザーの設定を返す。未保存なら domain.ErrNotFound。
func (r *UserPrefsRepo) Get(ctx context.Context, userID int64) (*domain.UserPrefs, error) {
	var (
		p         domain.UserPrefs
		updatedAt int64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT user_id, quiz_set, quiz_mode, updated_at FROM user_prefs WHERE user_id = ?`, userID).
		Scan(&p.UserID, &p.QuizSet, &p.QuizMode, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("user_prefs get: %w", err)
	}
	p.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &p, nil
}

// Upsert はユーザーの設定を挿入または更新する。
func (r *UserPrefsRepo) Upsert(ctx context.Context, p *domain.UserPrefs) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_prefs(user_id, quiz_set, quiz_mode, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		   quiz_set = excluded.quiz_set,
		   quiz_mode = excluded.quiz_mode,
		   updated_at = excluded.updated_at`,
		p.UserID, p.QuizSet, string(p.QuizMode), p.UpdatedAt.Unix())
	if err != nil {
		return fmt.Errorf("user_prefs upsert: %w", err)
	}
	return nil
}
