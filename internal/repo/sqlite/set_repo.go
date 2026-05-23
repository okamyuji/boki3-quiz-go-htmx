package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

// SetRepo は question_sets テーブルへのアクセスを提供する。
type SetRepo struct {
	db *sql.DB
}

// NewSetRepo は SetRepo を生成する。
func NewSetRepo(db *sql.DB) *SetRepo { return &SetRepo{db: db} }

var _ port.SetRepository = (*SetRepo)(nil)

// GetByCode は code で 1 件返す。
func (r *SetRepo) GetByCode(ctx context.Context, code string) (*domain.QuestionSet, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, code, name, description, target_size FROM question_sets WHERE code = ?`, code)
	var s domain.QuestionSet
	if err := row.Scan(&s.ID, &s.Code, &s.Name, &s.Description, &s.TargetSize); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("set get: %w", err)
	}
	return &s, nil
}

// ListAll は全 question_sets を id 昇順で返す。
func (r *SetRepo) ListAll(ctx context.Context) ([]domain.QuestionSet, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, code, name, description, target_size FROM question_sets ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("set list: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.QuestionSet, 0)
	for rows.Next() {
		var s domain.QuestionSet
		if err := rows.Scan(&s.ID, &s.Code, &s.Name, &s.Description, &s.TargetSize); err != nil {
			return nil, fmt.Errorf("set scan: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("set iter: %w", err)
	}
	return out, nil
}
