package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/srs"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

// SRSStateRepo は srs_states テーブルへのアクセスを提供する。
type SRSStateRepo struct {
	db *sql.DB
}

// NewSRSStateRepo は SRSStateRepo を生成する。
func NewSRSStateRepo(db *sql.DB) *SRSStateRepo { return &SRSStateRepo{db: db} }

var _ port.SRSStateRepository = (*SRSStateRepo)(nil)

const upsertSRSSQL = `INSERT INTO srs_states(
  user_id, question_id, efactor, interval_days, repetitions, due_at, last_grade, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id, question_id) DO UPDATE SET
  efactor=excluded.efactor,
  interval_days=excluded.interval_days,
  repetitions=excluded.repetitions,
  due_at=excluded.due_at,
  last_grade=excluded.last_grade,
  updated_at=excluded.updated_at`

// Upsert は SRS 状態を保存する。既存があれば置換する。
func (r *SRSStateRepo) Upsert(ctx context.Context, s *srs.State) error {
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, upsertSRSSQL,
		s.UserID, s.QuestionID, s.EFactor, s.IntervalDays, s.Repetitions,
		s.DueAt.Unix(), int(s.LastGrade), s.UpdatedAt.Unix())
	if err != nil {
		return fmt.Errorf("srs upsert: %w", err)
	}
	return nil
}

// DueForUser は due_at <= now の状態を due_at 昇順で返す。
func (r *SRSStateRepo) DueForUser(ctx context.Context, userID int64, now time.Time, limit int) ([]srs.State, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT user_id, question_id, efactor, interval_days, repetitions, due_at, last_grade, updated_at
		 FROM srs_states WHERE user_id = ? AND due_at <= ?
		 ORDER BY due_at ASC LIMIT ?`,
		userID, now.Unix(), limit)
	if err != nil {
		return nil, fmt.Errorf("srs due: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]srs.State, 0)
	for rows.Next() {
		s, err := scanSRSRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("srs iter: %w", err)
	}
	return out, nil
}

// Get は (user, question) の状態を返す。なければ ErrNotFound。
func (r *SRSStateRepo) Get(ctx context.Context, userID, questionID int64) (*srs.State, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT user_id, question_id, efactor, interval_days, repetitions, due_at, last_grade, updated_at
		 FROM srs_states WHERE user_id = ? AND question_id = ?`, userID, questionID)
	var s srs.State
	var due, updated int64
	var grade int
	if err := row.Scan(&s.UserID, &s.QuestionID, &s.EFactor, &s.IntervalDays, &s.Repetitions,
		&due, &grade, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("srs get: %w", err)
	}
	s.DueAt = time.Unix(due, 0).UTC()
	s.UpdatedAt = time.Unix(updated, 0).UTC()
	s.LastGrade = srs.Grade(grade)
	return &s, nil
}

// DeleteAllForUser はユーザの全 SRS 状態を削除する。
func (r *SRSStateRepo) DeleteAllForUser(ctx context.Context, userID int64) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM srs_states WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("srs delete all: %w", err)
	}
	return nil
}

// CountDueForUser は due_at <= now の件数を返す。
func (r *SRSStateRepo) CountDueForUser(ctx context.Context, userID int64, now time.Time) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM srs_states WHERE user_id = ? AND due_at <= ?`, userID, now.Unix(),
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("srs count due: %w", err)
	}
	return n, nil
}

func scanSRSRow(rows *sql.Rows) (srs.State, error) {
	var s srs.State
	var due, updated int64
	var grade int
	if err := rows.Scan(&s.UserID, &s.QuestionID, &s.EFactor, &s.IntervalDays, &s.Repetitions,
		&due, &grade, &updated); err != nil {
		return srs.State{}, fmt.Errorf("srs scan: %w", err)
	}
	s.DueAt = time.Unix(due, 0).UTC()
	s.UpdatedAt = time.Unix(updated, 0).UTC()
	s.LastGrade = srs.Grade(grade)
	return s, nil
}
