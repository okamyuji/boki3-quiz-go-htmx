package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

// TopicRepo は topics テーブルへの読取アクセス。
type TopicRepo struct {
	db *sql.DB
}

// NewTopicRepo は TopicRepo を生成する。
func NewTopicRepo(db *sql.DB) *TopicRepo { return &TopicRepo{db: db} }

var _ port.TopicRepository = (*TopicRepo)(nil)

// ListAll は topics を ord 昇順で返す。
func (r *TopicRepo) ListAll(ctx context.Context) ([]domain.Topic, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, code, name, ord FROM topics ORDER BY ord ASC`)
	if err != nil {
		return nil, fmt.Errorf("topics list: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.Topic, 0)
	for rows.Next() {
		var t domain.Topic
		if err := rows.Scan(&t.ID, &t.Code, &t.Name, &t.Ord); err != nil {
			return nil, fmt.Errorf("topic scan: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("topic iter: %w", err)
	}
	return out, nil
}
