package service

import (
	"context"
	"database/sql"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/srs"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/clock"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/sqlitex"
	reposqlite "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
)

// newPickSRSFixture は q1..q3 の 3 問と user を持つ DB を作る。
//   - q2: SRS で due
//   - q3: 不正解履歴あり (古い attempt)
//   - q1: 直近の正解 attempt (= lastQID)
func newPickSRSFixture(t *testing.T) (db *sql.DB, userID int64, qIDs [3]int64) {
	t.Helper()
	ctx := context.Background()
	var err error
	db, err = sqlitex.Open("file:" + filepath.Join(t.TempDir(), "picksrs.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := reposqlite.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO users(username, password_hash, password_salt, password_params, password_updated_at, created_at, updated_at)
		 VALUES ('srsuser', X'00', X'00', 't', 0, 0, 0)`)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	userID, _ = res.LastInsertId()
	if _, err := db.ExecContext(ctx, `INSERT INTO topics(code, name, ord) VALUES ('cash', '現金', 1)`); err != nil {
		t.Fatalf("insert topic: %v", err)
	}
	for i := range 3 {
		res, err := db.ExecContext(ctx,
			`INSERT INTO questions(code, topic_id, question_type, difficulty, prompt, payload_json, answer_json, explanation, references_json, created_at)
			 VALUES (?, 1, 'journal', 1, ?, '{}', '{}', 'e', NULL, 0)`,
			"srs-q"+string(rune('1'+i)), "p")
		if err != nil {
			t.Fatalf("insert question: %v", err)
		}
		qIDs[i], _ = res.LastInsertId()
	}
	// q2 を due にする (due_at 過去)。
	if _, err := db.ExecContext(ctx,
		`INSERT INTO srs_states(user_id, question_id, efactor, interval_days, repetitions, due_at, last_grade, updated_at)
		 VALUES (?, ?, 2.5, 0, 0, 1000, 0, 1000)`, userID, qIDs[1]); err != nil {
		t.Fatalf("insert srs: %v", err)
	}
	// q3 不正解 (古い) → q1 正解 (直近, lastQID になる)。
	for _, a := range []struct {
		qid       int64
		isCorrect int
		at        int64
	}{{qIDs[2], 0, 1000}, {qIDs[0], 1, 2000}} {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO attempts(user_id, question_id, set_id, is_correct, duration_ms, submitted_answer_json, answered_at)
			 VALUES (?, ?, NULL, ?, 1000, '{}', ?)`, userID, a.qid, a.isCorrect, a.at); err != nil {
			t.Fatalf("insert attempt: %v", err)
		}
	}
	return db, userID, qIDs
}

func newSRSQuizService(db *sql.DB, r *rand.Rand) *QuizService {
	svc := NewQuizService(
		reposqlite.NewQuestionRepo(db), reposqlite.NewSetRepo(db),
		reposqlite.NewAttemptRepo(db), reposqlite.NewSRSStateRepo(db),
		clock.Fixed{T: time.Unix(100000, 0).UTC()},
	)
	svc.rng = func() *rand.Rand { return r }
	return svc
}

// pickSRS の重み付き選定: due 70% / 不正解履歴 20% / ランダム 10%。
// シード固定の rng を共有するため試行列は決定的。範囲アサーションは
// 実装の重みが保たれていることを検証する。
func TestPickSRSWeightsDueThenIncorrectHistory(t *testing.T) {
	t.Parallel()
	db, userID, qIDs := newPickSRSFixture(t)
	r := rand.New(rand.NewPCG(7, 42))
	svc := newSRSQuizService(db, r)
	all := []domain.Question{{ID: qIDs[0]}, {ID: qIDs[1]}, {ID: qIDs[2]}}

	counts := map[int64]int{}
	const trials = 300
	for range trials {
		q, err := svc.pickSRS(context.Background(), userID, all)
		if err != nil {
			t.Fatalf("pickSRS: %v", err)
		}
		counts[q.ID]++
	}
	// シードと DB 内容が固定なので試行列は完全に決定的。
	// 期待値: due (q2) = 70% + ランダム落ち分 ≈ 75%、不正解履歴 (q3) ≈ 25%、直近出題の q1 は除外。
	// 重み定数・分岐条件の変化 (mutation) を検出するため実測値で厳密に固定する。
	if c := counts[qIDs[1]]; c != 228 {
		t.Fatalf("due question picked %d/%d, want exactly 228 (deterministic seed)", c, trials)
	}
	if c := counts[qIDs[2]]; c != 72 {
		t.Fatalf("incorrect-history question picked %d/%d, want exactly 72 (deterministic seed)", c, trials)
	}
	if c := counts[qIDs[0]]; c != 0 {
		t.Fatalf("last question picked %d times, want 0 (must be avoided)", c)
	}
}

// due が空かつ履歴なしならランダムフォールバックで全問から選ぶ。
func TestPickSRSFallsBackToRandomWhenNoDueAndNoHistory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := sqlitex.Open("file:" + filepath.Join(t.TempDir(), "fallback.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := reposqlite.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO users(username, password_hash, password_salt, password_params, password_updated_at, created_at, updated_at)
		 VALUES ('u', X'00', X'00', 't', 0, 0, 0)`)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	uid, _ := res.LastInsertId()

	r := rand.New(rand.NewPCG(1, 2))
	svc := newSRSQuizService(db, r)
	all := []domain.Question{{ID: 501}}
	q, err := svc.pickSRS(ctx, uid, all)
	if err != nil {
		t.Fatalf("pickSRS: %v", err)
	}
	if q.ID != 501 {
		t.Fatalf("q.ID = %d, want 501 (only candidate)", q.ID)
	}
}

// DueForUser のエラーは呼び出し元へ伝播する。
type errSRSRepo struct{}

func (errSRSRepo) Upsert(context.Context, *srs.State) error { return context.DeadlineExceeded }
func (errSRSRepo) DueForUser(context.Context, int64, time.Time, int) ([]srs.State, error) {
	return nil, context.DeadlineExceeded
}
func (errSRSRepo) Get(context.Context, int64, int64) (*srs.State, error) {
	return nil, context.DeadlineExceeded
}
func (errSRSRepo) DeleteAllForUser(context.Context, int64) error { return context.DeadlineExceeded }
func (errSRSRepo) CountDueForUser(context.Context, int64, time.Time) (int, error) {
	return 0, context.DeadlineExceeded
}

func TestPickSRSPropagatesDueLookupError(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(1, 2))
	svc := &QuizService{srs: errSRSRepo{}, clock: clock.Fixed{T: time.Unix(0, 0)},
		rng: func() *rand.Rand { return r }}
	_, err := svc.pickSRS(context.Background(), 1, []domain.Question{{ID: 1}})
	if err == nil || !strings.Contains(err.Error(), "quiz srs due") {
		t.Fatalf("err = %v, want wrapped 'quiz srs due' error", err)
	}
}
