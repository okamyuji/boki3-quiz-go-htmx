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

// QuestionRepo は questions テーブルへのアクセスを提供する。
type QuestionRepo struct {
	db *sql.DB
}

// NewQuestionRepo は QuestionRepo を生成する。
func NewQuestionRepo(db *sql.DB) *QuestionRepo { return &QuestionRepo{db: db} }

var _ port.QuestionRepository = (*QuestionRepo)(nil)

const selectQuestionBaseSQL = `SELECT q.id, q.code, q.topic_id, q.question_type, q.difficulty,
  q.prompt, q.payload_json, q.answer_json, q.explanation, COALESCE(q.references_json, ''), q.created_at
  FROM questions q`

// GetByID は id で 1 件返す。
func (r *QuestionRepo) GetByID(ctx context.Context, id int64) (*domain.Question, error) {
	row := r.db.QueryRowContext(ctx, selectQuestionBaseSQL+" WHERE q.id = ?", id)
	return scanQuestion(row)
}

// GetByCode は code で 1 件返す。
func (r *QuestionRepo) GetByCode(ctx context.Context, code string) (*domain.Question, error) {
	row := r.db.QueryRowContext(ctx, selectQuestionBaseSQL+" WHERE q.code = ?", code)
	return scanQuestion(row)
}

// ListBySet は question_set_members 経由で当該セットの全問題を ord 昇順で返す。
func (r *QuestionRepo) ListBySet(ctx context.Context, setCode string) ([]domain.Question, error) {
	const q = `SELECT q.id, q.code, q.topic_id, q.question_type, q.difficulty,
		q.prompt, q.payload_json, q.answer_json, q.explanation, COALESCE(q.references_json, ''), q.created_at
		FROM questions q
		JOIN question_set_members m ON m.question_id = q.id
		JOIN question_sets s ON s.id = m.set_id
		WHERE s.code = ?
		ORDER BY m.ord ASC, q.id ASC`
	rows, err := r.db.QueryContext(ctx, q, setCode)
	if err != nil {
		return nil, fmt.Errorf("question list by set: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanQuestions(rows)
}

// Search は filter に従う検索結果を返す。
//
// 動的 SQL を構築するが、値はすべて ? プレースホルダで渡す (fmt.Sprintf で値を埋め込まない)。
func (r *QuestionRepo) Search(ctx context.Context, filter domain.QuestionFilter) ([]domain.Question, error) {
	var sb strings.Builder
	sb.WriteString(`SELECT q.id, q.code, q.topic_id, q.question_type, q.difficulty,
		q.prompt, q.payload_json, q.answer_json, q.explanation, COALESCE(q.references_json, ''), q.created_at
		FROM questions q`)

	args := make([]any, 0)
	conds := make([]string, 0)

	if filter.SetCode != "" {
		sb.WriteString(` JOIN question_set_members m ON m.question_id = q.id
			JOIN question_sets s ON s.id = m.set_id`)
		conds = append(conds, "s.code = ?")
		args = append(args, filter.SetCode)
	}
	if len(filter.TopicCodes) > 0 {
		placeholders := strings.Repeat("?,", len(filter.TopicCodes))
		placeholders = placeholders[:len(placeholders)-1]
		conds = append(conds, "q.topic_id IN (SELECT id FROM topics WHERE code IN ("+placeholders+"))")
		for _, c := range filter.TopicCodes {
			args = append(args, c)
		}
	}
	if len(filter.Types) > 0 {
		placeholders := strings.Repeat("?,", len(filter.Types))
		placeholders = placeholders[:len(placeholders)-1]
		conds = append(conds, "q.question_type IN ("+placeholders+")")
		for _, t := range filter.Types {
			args = append(args, string(t))
		}
	}
	if len(conds) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conds, " AND "))
	}
	sb.WriteString(" ORDER BY q.id ASC")
	if filter.Limit > 0 {
		sb.WriteString(" LIMIT ?")
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		sb.WriteString(" OFFSET ?")
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("question search: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanQuestions(rows)
}

func scanQuestion(row *sql.Row) (*domain.Question, error) {
	var q domain.Question
	var created int64
	var qt string
	if err := row.Scan(&q.ID, &q.Code, &q.TopicID, &qt, &q.Difficulty,
		&q.Prompt, &q.PayloadJSON, &q.AnswerJSON, &q.Explanation, &q.ReferencesJSON, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("question scan: %w", err)
	}
	q.QuestionType = domain.QuestionType(qt)
	q.CreatedAt = time.Unix(created, 0).UTC()
	return &q, nil
}

func scanQuestions(rows *sql.Rows) ([]domain.Question, error) {
	out := make([]domain.Question, 0)
	for rows.Next() {
		var q domain.Question
		var created int64
		var qt string
		if err := rows.Scan(&q.ID, &q.Code, &q.TopicID, &qt, &q.Difficulty,
			&q.Prompt, &q.PayloadJSON, &q.AnswerJSON, &q.Explanation, &q.ReferencesJSON, &created); err != nil {
			return nil, fmt.Errorf("question scan: %w", err)
		}
		q.QuestionType = domain.QuestionType(qt)
		q.CreatedAt = time.Unix(created, 0).UTC()
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("question iter: %w", err)
	}
	return out, nil
}
