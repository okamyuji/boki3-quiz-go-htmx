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

// AttemptRepo は attempts テーブルへのアクセスと集計を提供する。
type AttemptRepo struct {
	db *sql.DB
}

// NewAttemptRepo は AttemptRepo を生成する。
func NewAttemptRepo(db *sql.DB) *AttemptRepo { return &AttemptRepo{db: db} }

var _ port.AttemptRepository = (*AttemptRepo)(nil)

const insertAttemptSQL = `INSERT INTO attempts(
  user_id, question_id, set_id, is_correct, duration_ms, submitted_answer_json, answered_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`

// Create は attempts に 1 件追加する。a.ID が更新される。
func (r *AttemptRepo) Create(ctx context.Context, a *domain.Attempt) error {
	if a.AnsweredAt.IsZero() {
		a.AnsweredAt = time.Now().UTC()
	}
	var setIDArg any
	if a.SetID != nil {
		setIDArg = *a.SetID
	}
	correct := 0
	if a.IsCorrect {
		correct = 1
	}
	res, err := r.db.ExecContext(ctx, insertAttemptSQL,
		a.UserID, a.QuestionID, setIDArg, correct, a.DurationMs, a.SubmittedAnswerJSON, a.AnsweredAt.Unix())
	if err != nil {
		return fmt.Errorf("attempt create: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("attempt last id: %w", err)
	}
	a.ID = id
	return nil
}

// ListByUser は user の attempts を answered_at 降順で limit/offset 適用して返す。
func (r *AttemptRepo) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.Attempt, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, question_id, set_id, is_correct, duration_ms, submitted_answer_json, answered_at
		 FROM attempts WHERE user_id = ? ORDER BY answered_at DESC, id DESC LIMIT ? OFFSET ?`,
		userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("attempt list: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.Attempt, 0)
	for rows.Next() {
		var a domain.Attempt
		var setID sql.NullInt64
		var correct int
		var answered int64
		if err := rows.Scan(&a.ID, &a.UserID, &a.QuestionID, &setID, &correct,
			&a.DurationMs, &a.SubmittedAnswerJSON, &answered); err != nil {
			return nil, fmt.Errorf("attempt scan: %w", err)
		}
		if setID.Valid {
			v := setID.Int64
			a.SetID = &v
		}
		a.IsCorrect = correct != 0
		a.AnsweredAt = time.Unix(answered, 0).UTC()
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("attempt iter: %w", err)
	}
	return out, nil
}

// DeleteByID は (userID, attemptID) が一致するレコードを削除する。
//
// 削除対象が存在しないか他人の attempt の場合は ErrNotFound を返す。
// 上位 (handler) は ErrNotFound と「真に存在しない」を区別せず 404 で返すことで列挙を防ぐ。
func (r *AttemptRepo) DeleteByID(ctx context.Context, userID, attemptID int64) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM attempts WHERE id = ? AND user_id = ?`, attemptID, userID)
	if err != nil {
		return fmt.Errorf("attempt delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("attempt delete rows: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeleteAllForUser はユーザの全 attempt を削除する。
func (r *AttemptRepo) DeleteAllForUser(ctx context.Context, userID int64) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM attempts WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("attempt delete all: %w", err)
	}
	return nil
}

// StatsByTopic は論点別の合計と正解数を返す。
func (r *AttemptRepo) StatsByTopic(ctx context.Context, userID int64) ([]domain.TopicStat, error) {
	const q = `SELECT t.code, t.name, COUNT(a.id), COALESCE(SUM(a.is_correct), 0)
		FROM topics t
		LEFT JOIN questions q ON q.topic_id = t.id
		LEFT JOIN attempts a ON a.question_id = q.id AND a.user_id = ?
		GROUP BY t.id
		ORDER BY t.ord ASC`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("attempt stats by topic: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.TopicStat, 0)
	for rows.Next() {
		var s domain.TopicStat
		if err := rows.Scan(&s.TopicCode, &s.TopicName, &s.Total, &s.Correct); err != nil {
			return nil, fmt.Errorf("topic stat scan: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("topic stat iter: %w", err)
	}
	return out, nil
}

// DailyAccuracy は now を UTC 日に切り、過去 days 日 (now を含む) の attempts を集計して返す。
// データが無い日は欠落 (UI 側で 0 埋め)。
func (r *AttemptRepo) DailyAccuracy(ctx context.Context, userID int64, days int, now time.Time) ([]domain.DailyAccuracy, error) {
	if days <= 0 {
		days = 7
	}
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(days - 1))
	const q = `SELECT strftime('%Y-%m-%d', answered_at, 'unixepoch') AS d,
		COUNT(*), COALESCE(SUM(is_correct), 0)
		FROM attempts
		WHERE user_id = ? AND answered_at >= ?
		GROUP BY d
		ORDER BY d ASC`
	rows, err := r.db.QueryContext(ctx, q, userID, startDay.Unix())
	if err != nil {
		return nil, fmt.Errorf("daily accuracy: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.DailyAccuracy, 0)
	for rows.Next() {
		var d string
		var da domain.DailyAccuracy
		if err := rows.Scan(&d, &da.Total, &da.Correct); err != nil {
			return nil, fmt.Errorf("daily scan: %w", err)
		}
		ts, err := time.Parse("2006-01-02", d)
		if err != nil {
			return nil, fmt.Errorf("daily parse: %w", err)
		}
		da.Date = ts.UTC()
		out = append(out, da)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("daily iter: %w", err)
	}
	return out, nil
}

// LastQuestionIDInSet は当該セット内でユーザが最後に回答した問題 ID を返す。
// 回答がなければ domain.ErrNotFound。
func (r *AttemptRepo) LastQuestionIDInSet(ctx context.Context, userID, setID int64) (int64, error) {
	var qid int64
	err := r.db.QueryRowContext(ctx,
		`SELECT question_id FROM attempts WHERE user_id = ? AND set_id = ?
		 ORDER BY answered_at DESC, id DESC LIMIT 1`, userID, setID,
	).Scan(&qid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("attempt last in set: %w", err)
	}
	return qid, nil
}

// WeakTopicIDs は since 以降の回答を論点別に集計し、誤答を 1 件以上含む論点を
// 誤答率降順 (同率なら topic_id 昇順) で最大 limit 件返す。
func (r *AttemptRepo) WeakTopicIDs(ctx context.Context, userID int64, since time.Time, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 3
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT q.topic_id
		 FROM attempts a JOIN questions q ON q.id = a.question_id
		 WHERE a.user_id = ? AND a.answered_at >= ?
		 GROUP BY q.topic_id
		 HAVING SUM(1 - a.is_correct) > 0
		 ORDER BY CAST(SUM(1 - a.is_correct) AS REAL) / COUNT(*) DESC, q.topic_id ASC
		 LIMIT ?`, userID, since.Unix(), limit)
	if err != nil {
		return nil, fmt.Errorf("attempt weak topics: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanInt64s(rows, "weak topic")
}

// AttemptedQuestionIDs はユーザが 1 回以上回答した問題 ID を昇順で返す。
func (r *AttemptRepo) AttemptedQuestionIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT question_id FROM attempts WHERE user_id = ? ORDER BY question_id ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("attempt attempted ids: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanInt64s(rows, "attempted id")
}

// scanInt64s は単一 int64 列の rows を読み切って返す。
func scanInt64s(rows *sql.Rows, label string) ([]int64, error) {
	out := make([]int64, 0)
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("%s scan: %w", label, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s iter: %w", label, err)
	}
	return out, nil
}

// SummaryForUser は全試行数と正解数を返す。
func (r *AttemptRepo) SummaryForUser(ctx context.Context, userID int64) (totalAttempts, totalCorrect int, err error) {
	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(is_correct), 0) FROM attempts WHERE user_id = ?`, userID,
	).Scan(&totalAttempts, &totalCorrect)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, 0, fmt.Errorf("attempt summary: %w", err)
	}
	return totalAttempts, totalCorrect, nil
}
