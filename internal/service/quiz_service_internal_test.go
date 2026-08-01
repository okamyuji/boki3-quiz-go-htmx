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

// pickSRS の重み付き選定: due 70% / 弱点論点 20% / 未着手 10%。
// フィクスチャでは q2 が due かつ唯一の未着手、q3 が誤答済み (弱点論点候補)、
// q1 が直近出題 (除外)。q3 は弱点論点枠 (20% の約半分) でのみ選ばれる。
// シード固定の rng を共有するため試行列は完全に決定的。
// 重み定数・分岐条件の変化 (mutation) を検出するため実測値で厳密に固定する。
func TestPickSRSWeightsDueThenWeakTopicThenUnattempted(t *testing.T) {
	t.Parallel()
	db, userID, qIDs := newPickSRSFixture(t)
	r := rand.New(rand.NewPCG(7, 42))
	svc := newSRSQuizService(db, r)
	all := []domain.Question{
		{ID: qIDs[0], TopicID: 1}, {ID: qIDs[1], TopicID: 1}, {ID: qIDs[2], TopicID: 1},
	}

	counts := map[int64]int{}
	const trials = 300
	for range trials {
		q, err := svc.pickSRS(context.Background(), userID, all)
		if err != nil {
			t.Fatalf("pickSRS: %v", err)
		}
		counts[q.ID]++
	}
	// 期待値: q2 ≈ 70% (due) + 20%/2 (弱点論点) + 10% (未着手) ≈ 90%、q3 ≈ 10%。
	if c := counts[qIDs[1]]; c != 266 {
		t.Fatalf("due+unattempted question picked %d/%d, want exactly 266 (deterministic seed)", c, trials)
	}
	if c := counts[qIDs[2]]; c != 34 {
		t.Fatalf("weak-topic question picked %d/%d, want exactly 34 (deterministic seed)", c, trials)
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

// 未着手問題の優先: due なし・誤答なしのとき、未回答の問題が必ず選ばれる。
func TestPickSRSPicksUnattemptedWhenNoDueNoWeakTopic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := sqlitex.Open("file:" + filepath.Join(t.TempDir(), "unattempted.db"))
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
	if _, err := db.ExecContext(ctx, `INSERT INTO topics(code, name, ord) VALUES ('cash', '現金', 1)`); err != nil {
		t.Fatalf("insert topic: %v", err)
	}
	var qIDs [3]int64
	for i := range 3 {
		res, err := db.ExecContext(ctx,
			`INSERT INTO questions(code, topic_id, question_type, difficulty, prompt, payload_json, answer_json, explanation, references_json, created_at)
			 VALUES (?, 1, 'journal', 1, 'p', '{}', '{}', 'e', NULL, 0)`, "ua-q"+string(rune('1'+i)))
		if err != nil {
			t.Fatalf("insert question: %v", err)
		}
		qIDs[i], _ = res.LastInsertId()
	}
	// q3 を過去に正解、q1 を直近に正解 (lastQID)。q2 だけ未回答。誤答ゼロ。
	for _, a := range []struct {
		qid int64
		at  int64
	}{{qIDs[2], 1000}, {qIDs[0], 2000}} {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO attempts(user_id, question_id, set_id, is_correct, duration_ms, submitted_answer_json, answered_at)
			 VALUES (?, ?, NULL, 1, 1000, '{}', ?)`, uid, a.qid, a.at); err != nil {
			t.Fatalf("insert attempt: %v", err)
		}
	}
	r := rand.New(rand.NewPCG(3, 9))
	svc := newSRSQuizService(db, r)
	all := []domain.Question{
		{ID: qIDs[0], TopicID: 1}, {ID: qIDs[1], TopicID: 1}, {ID: qIDs[2], TopicID: 1},
	}
	for i := range 50 {
		q, err := svc.pickSRS(ctx, uid, all)
		if err != nil {
			t.Fatalf("pickSRS: %v", err)
		}
		if q.ID != qIDs[1] {
			t.Fatalf("trial %d: got qID=%d, want %d (未着手問題が常に優先されるべき)", i, q.ID, qIDs[1])
		}
	}
}

// 弱点論点 (過去 7 日の誤答率上位) に属する問題は、誤答した問題そのものに
// 限らず同論点の未回答問題も候補になる。未着手枠と合わせ、誤答済み問題より
// 未回答問題の方が多く選ばれる分布になる。
func TestPickSRSWeakTopicIncludesUnansweredQuestionOfWeakTopic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := sqlitex.Open("file:" + filepath.Join(t.TempDir(), "weaktopic.db"))
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
	for _, tp := range []struct {
		code string
		ord  int
	}{{"weak", 1}, {"strong", 2}, {"weak2", 3}} {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO topics(code, name, ord) VALUES (?, ?, ?)`, tp.code, tp.code, tp.ord); err != nil {
			t.Fatalf("insert topic: %v", err)
		}
	}
	insertQ := func(code string, topicID int64) int64 {
		res, err := db.ExecContext(ctx,
			`INSERT INTO questions(code, topic_id, question_type, difficulty, prompt, payload_json, answer_json, explanation, references_json, created_at)
			 VALUES (?, ?, 'journal', 1, 'p', '{}', '{}', 'e', NULL, 0)`, code, topicID)
		if err != nil {
			t.Fatalf("insert question %s: %v", code, err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	qA := insertQ("wt-qa", 1) // 弱点論点 1・誤答済み
	qB := insertQ("wt-qb", 1) // 弱点論点 1・未回答
	qX := insertQ("wt-qx", 2) // 強い論点・直近正解 (lastQID)
	qC := insertQ("wt-qc", 3) // 弱点論点 2 (2 位)・誤答済み
	for _, a := range []struct {
		qid     int64
		correct int
		at      int64
	}{{qA, 0, 1000}, {qC, 0, 1500}, {qX, 1, 2000}} {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO attempts(user_id, question_id, set_id, is_correct, duration_ms, submitted_answer_json, answered_at)
			 VALUES (?, ?, NULL, ?, 1000, '{}', ?)`, uid, a.qid, a.correct, a.at); err != nil {
			t.Fatalf("insert attempt: %v", err)
		}
	}
	r := rand.New(rand.NewPCG(11, 13))
	svc := newSRSQuizService(db, r)
	all := []domain.Question{
		{ID: qA, TopicID: 1}, {ID: qB, TopicID: 1}, {ID: qX, TopicID: 2}, {ID: qC, TopicID: 3},
	}
	counts := map[int64]int{}
	const trials = 300
	for range trials {
		q, err := svc.pickSRS(ctx, uid, all)
		if err != nil {
			t.Fatalf("pickSRS: %v", err)
		}
		counts[q.ID]++
	}
	// シード・DB 内容が固定なので試行列は完全に決定的。重み定数・候補集合の
	// 変化 (mutation) を検出するため実測値で厳密に固定する。
	// 概算: 弱点論点枠 2/3 で {qA,qB,qC} 等確率、未着手枠 1/3 で qB のみ。
	if counts[qX] != 0 {
		t.Fatalf("lastQID qX picked %d times, want 0", counts[qX])
	}
	if counts[qA] != 68 || counts[qB] != 171 || counts[qC] != 61 {
		t.Fatalf("counts qA=%d qB=%d qC=%d, want exactly 68/171/61 (deterministic seed)",
			counts[qA], counts[qB], counts[qC])
	}
	if counts[qB] <= counts[qA] {
		t.Fatalf("qB(未回答・弱点論点+未着手枠)=%d <= qA(誤答済み)=%d — 未回答問題が多数派になるべき", counts[qB], counts[qA])
	}
}

// 弱点論点/未着手の照会が失敗しても選定はランダムフォールバックで継続する。
type errAttemptsRepo struct{}

func (errAttemptsRepo) Create(context.Context, *domain.Attempt) error { return nil }
func (errAttemptsRepo) ListByUser(context.Context, int64, int, int) ([]domain.Attempt, error) {
	return nil, nil
}
func (errAttemptsRepo) DeleteByID(context.Context, int64, int64) error { return nil }
func (errAttemptsRepo) DeleteAllForUser(context.Context, int64) error  { return nil }
func (errAttemptsRepo) StatsByTopic(context.Context, int64) ([]domain.TopicStat, error) {
	return nil, nil
}
func (errAttemptsRepo) DailyAccuracy(context.Context, int64, int, time.Time) ([]domain.DailyAccuracy, error) {
	return nil, nil
}
func (errAttemptsRepo) SummaryForUser(context.Context, int64) (totalAttempts, totalCorrect int, err error) {
	return 0, 0, nil
}
func (errAttemptsRepo) LastQuestionIDInSet(context.Context, int64, int64) (int64, error) {
	return 0, context.DeadlineExceeded
}
func (errAttemptsRepo) WeakTopicIDs(context.Context, int64, time.Time, int) ([]int64, error) {
	return nil, context.DeadlineExceeded
}
func (errAttemptsRepo) AttemptedQuestionIDs(context.Context, int64) ([]int64, error) {
	return nil, context.DeadlineExceeded
}

type emptySRSRepo struct{ errSRSRepo }

func (emptySRSRepo) DueForUser(context.Context, int64, time.Time, int) ([]srs.State, error) {
	return nil, nil
}

func TestPickSRSFallsBackToRandomWhenWeakAndUnattemptedLookupsFail(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(5, 6))
	svc := &QuizService{
		attempts: errAttemptsRepo{}, srs: emptySRSRepo{},
		clock: clock.Fixed{T: time.Unix(100000, 0).UTC()},
		rng:   func() *rand.Rand { return r },
	}
	all := []domain.Question{{ID: 1, TopicID: 1}, {ID: 2, TopicID: 1}}
	q, err := svc.pickSRS(context.Background(), 42, all)
	if err != nil {
		t.Fatalf("pickSRS: %v", err)
	}
	if q == nil {
		t.Fatalf("q is nil, want random fallback question")
	}
}

// mapQuestionsRepo は GetByID のみ実体を持つ questions リポジトリのフェイク。
type mapQuestionsRepo map[int64]domain.Question

func (m mapQuestionsRepo) GetByID(_ context.Context, id int64) (*domain.Question, error) {
	if q, ok := m[id]; ok {
		return &q, nil
	}
	return nil, domain.ErrNotFound
}
func (m mapQuestionsRepo) GetByCode(context.Context, string) (*domain.Question, error) {
	return nil, domain.ErrNotFound
}
func (m mapQuestionsRepo) ListBySet(context.Context, string) ([]domain.Question, error) {
	return nil, nil
}
func (m mapQuestionsRepo) Search(context.Context, domain.QuestionFilter) ([]domain.Question, error) {
	return nil, nil
}

// due に lastQID 以外の候補がある限り、必ずそれを返す (nil にならない)。
func TestPickDueAvoidAlwaysReturnsNonLastCandidate(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(21, 22))
	svc := &QuizService{
		questions: mapQuestionsRepo{1: {ID: 1}, 2: {ID: 2}},
		rng:       func() *rand.Rand { return r },
	}
	due := []srs.State{{QuestionID: 1}, {QuestionID: 2}}
	for i := range 100 {
		q := svc.pickDueAvoid(context.Background(), due, 1, r)
		if q == nil {
			t.Fatalf("trial %d: got nil, want q2 (有効候補があるのに nil は不可)", i)
		}
		if q.ID != 2 {
			t.Fatalf("trial %d: got qID=%d, want 2 (lastQID=1 は回避)", i, q.ID)
		}
	}
}

// due が lastQID しか含まない場合はそれを返す (連続出題を許容)。
func TestPickDueAvoidReturnsLastWhenOnlyCandidate(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(23, 24))
	svc := &QuizService{
		questions: mapQuestionsRepo{1: {ID: 1}},
		rng:       func() *rand.Rand { return r },
	}
	q := svc.pickDueAvoid(context.Background(), []srs.State{{QuestionID: 1}}, 1, r)
	if q == nil || q.ID != 1 {
		t.Fatalf("got %v, want qID=1 (唯一の due 候補)", q)
	}
}

// 参照先の問題がすべて取得できない場合は nil (次の枠へフォールバック)。
func TestPickDueAvoidNilWhenQuestionsMissing(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(25, 26))
	svc := &QuizService{
		questions: mapQuestionsRepo{},
		rng:       func() *rand.Rand { return r },
	}
	due := []srs.State{{QuestionID: 8}, {QuestionID: 9}}
	if q := svc.pickDueAvoid(context.Background(), due, 1, r); q != nil {
		t.Fatalf("got %v, want nil (全候補の問題が欠落)", q)
	}
}

// pickRandomAvoid は lastQID を決して返さない (他候補がある場合)。
func TestPickRandomAvoidNeverPicksLast(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(27, 28))
	all := []domain.Question{{ID: 1}, {ID: 2}, {ID: 3}}
	for i := range 300 {
		q := pickRandomAvoid(all, 2, r)
		if q.ID == 2 {
			t.Fatalf("trial %d: picked lastQID=2 (回避されるべき)", i)
		}
	}
}

// 候補がすべて lastQID の場合 (要素 1 個/重複) は lastQID を返す。
func TestPickRandomAvoidReturnsLastWhenNoOtherOption(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(29, 30))
	single := []domain.Question{{ID: 5}}
	if q := pickRandomAvoid(single, 5, r); q.ID != 5 {
		t.Fatalf("single: got %d, want 5", q.ID)
	}
	dup := []domain.Question{{ID: 5}, {ID: 5}}
	if q := pickRandomAvoid(dup, 5, r); q.ID != 5 {
		t.Fatalf("dup: got %d, want 5", q.ID)
	}
}

// pickSequential はセット照会/最終回答照会の失敗を呼び出し元へ伝播する。
func TestPickSequentialPropagatesErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	all := []domain.Question{{ID: 1}}

	// セット照会失敗 (クローズ済み DB)。
	closedDB, err := sqlitex.Open("file:" + filepath.Join(t.TempDir(), "closed.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = closedDB.Close()
	svc := &QuizService{sets: reposqlite.NewSetRepo(closedDB)}
	if _, err := svc.pickSequential(ctx, 1, "core_300", all); err == nil || !strings.Contains(err.Error(), "quiz sequential set") {
		t.Fatalf("set error = %v, want wrapped 'quiz sequential set'", err)
	}

	// 最終回答照会失敗 (セットは実在、attempts が失敗)。
	db, err := sqlitex.Open("file:" + filepath.Join(t.TempDir(), "seq.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := reposqlite.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO question_sets(code, name, description, target_size) VALUES ('core_300', 'コア', '', 300)`); err != nil {
		t.Fatalf("insert set: %v", err)
	}
	svc2 := &QuizService{sets: reposqlite.NewSetRepo(db), attempts: errAttemptsRepo{}}
	if _, err := svc2.pickSequential(ctx, 1, "core_300", all); err == nil || !strings.Contains(err.Error(), "quiz sequential last") {
		t.Fatalf("last error = %v, want wrapped 'quiz sequential last'", err)
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
