package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/grading"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/srs"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/clock"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

// QuizService は port.QuizService の実装。
type QuizService struct {
	questions port.QuestionRepository
	sets      port.SetRepository
	attempts  port.AttemptRepository
	srs       port.SRSStateRepository
	clock     clock.Clock
	rng       func() *rand.Rand
}

// NewQuizService は QuizService を生成する。
func NewQuizService(
	q port.QuestionRepository, s port.SetRepository,
	a port.AttemptRepository, st port.SRSStateRepository,
	clk clock.Clock,
) *QuizService {
	if clk == nil {
		clk = clock.System{}
	}
	return &QuizService{
		questions: q, sets: s, attempts: a, srs: st, clock: clk,
		// 学習問題の選定にしか使わないため、暗号用途ではない math/rand/v2 で十分。
		rng: func() *rand.Rand { return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) }, //nolint:gosec // SRS 抽選用、暗号目的ではない
	}
}

var _ port.QuizService = (*QuizService)(nil)

// NextQuestion は学習モードに従い次の問題を返す。
//
//   - SRS:  due の問題から 70%、不正解履歴から 20%、ランダムから 10% の重み付き選択。
//   - Sequential: setCode のメンバーから次の id 順で。
//   - Random: setCode メンバーから無作為。
func (s *QuizService) NextQuestion(ctx context.Context, userID int64, setCode string, mode domain.QuizMode) (*domain.Question, error) {
	all, err := s.questions.ListBySet(ctx, setCode)
	if err != nil {
		return nil, fmt.Errorf("quiz next list: %w", err)
	}
	if len(all) == 0 {
		return nil, domain.ErrNotFound
	}
	switch mode {
	case domain.QuizModeRandom:
		r := s.rng()
		return &all[r.IntN(len(all))], nil
	case domain.QuizModeSequential:
		return s.pickSequential(ctx, userID, setCode, all)
	case domain.QuizModeSRS, "":
		return s.pickSRS(ctx, userID, all)
	default:
		return nil, domain.ErrInvalidInput
	}
}

// pickSequential はセット内でユーザが最後に回答した問題の次 (ord 順) を返す。
// 末尾まで回答したら先頭に戻る。回答履歴がない場合と、最後に回答した問題が
// セットから外れている場合は先頭を返す。
func (s *QuizService) pickSequential(ctx context.Context, userID int64, setCode string, all []domain.Question) (*domain.Question, error) {
	set, err := s.sets.GetByCode(ctx, setCode)
	if err != nil {
		return nil, fmt.Errorf("quiz sequential set: %w", err)
	}
	lastQID, err := s.attempts.LastQuestionIDInSet(ctx, userID, set.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return &all[0], nil
	}
	if err != nil {
		return nil, fmt.Errorf("quiz sequential last: %w", err)
	}
	for i := range all {
		if all[i].ID == lastQID {
			return &all[(i+1)%len(all)], nil
		}
	}
	return &all[0], nil
}

// 弱点論点の集計窓と採用論点数。
const (
	weakTopicWindowDays = 7
	weakTopicLimit      = 3
)

// pickSRS は学習モード SRS の選定ロジック。
//
// 既定重み: due 70% / 過去 7 日の誤答率上位論点 20% / 未着手問題 10%。
// due が空の場合は残り 30% を 20:10 比で再分配 (弱点論点 66.6% / 未着手 33.3%)。
// 弱点論点が空なら未着手へ、未着手も空なら全問ランダムに落ちる。
// いずれの枠でも直前に出題した問題 (lastQID) は避ける。
func (s *QuizService) pickSRS(ctx context.Context, userID int64, all []domain.Question) (*domain.Question, error) {
	now := s.clock.Now()
	due, err := s.srs.DueForUser(ctx, userID, now, 50)
	if err != nil {
		return nil, fmt.Errorf("quiz srs due: %w", err)
	}
	var lastQID int64
	if recent, _ := s.attempts.ListByUser(ctx, userID, 1, 0); len(recent) > 0 {
		lastQID = recent[0].QuestionID
	}
	r := s.rng()

	if len(due) > 0 && r.Float64() < 0.7 {
		if q := s.pickDueAvoid(ctx, due, lastQID, r); q != nil {
			return q, nil
		}
	}
	if r.Float64() < 2.0/3.0 {
		if q := s.pickWeakTopicAvoid(ctx, userID, all, lastQID, now, r); q != nil {
			return q, nil
		}
	}
	if q := s.pickUnattempted(ctx, userID, all, r); q != nil {
		return q, nil
	}
	return pickRandomAvoid(all, lastQID, r), nil
}

// pickWeakTopicAvoid は過去 7 日の誤答率上位論点に属する問題から lastQID 以外を
// 等確率で 1 件返す。照会失敗や候補なしは nil (次の枠へフォールバック)。
func (s *QuizService) pickWeakTopicAvoid(ctx context.Context, userID int64, all []domain.Question, lastQID int64, now time.Time, r *rand.Rand) *domain.Question {
	topics, err := s.attempts.WeakTopicIDs(ctx, userID, now.AddDate(0, 0, -weakTopicWindowDays), weakTopicLimit)
	if err != nil || len(topics) == 0 {
		return nil
	}
	weak := make(map[int64]bool, len(topics))
	for _, id := range topics {
		weak[id] = true
	}
	return pickCandidate(all, r, func(q *domain.Question) bool {
		return weak[q.TopicID] && q.ID != lastQID
	})
}

// pickUnattempted は未回答の問題から等確率で 1 件返す。
// 直近出題問題 (lastQID) は定義上回答済みなので候補に入らない。
// 照会失敗や候補なしは nil (ランダムへフォールバック)。
func (s *QuizService) pickUnattempted(ctx context.Context, userID int64, all []domain.Question, r *rand.Rand) *domain.Question {
	ids, err := s.attempts.AttemptedQuestionIDs(ctx, userID)
	if err != nil {
		return nil
	}
	attempted := make(map[int64]bool, len(ids))
	for _, id := range ids {
		attempted[id] = true
	}
	return pickCandidate(all, r, func(q *domain.Question) bool {
		return !attempted[q.ID]
	})
}

// pickCandidate は cond を満たす問題から等確率で 1 件返す (候補なしは nil)。
func pickCandidate(all []domain.Question, r *rand.Rand, cond func(*domain.Question) bool) *domain.Question {
	cands := make([]*domain.Question, 0, len(all))
	for i := range all {
		if cond(&all[i]) {
			cands = append(cands, &all[i])
		}
	}
	if len(cands) == 0 {
		return nil
	}
	return cands[r.IntN(len(cands))]
}

// pickDueAvoid は due から lastQID 以外を 1 件返す (見つからなければ nil)。
func (s *QuizService) pickDueAvoid(ctx context.Context, due []srs.State, lastQID int64, r *rand.Rand) *domain.Question {
	for range due {
		st := due[r.IntN(len(due))]
		if st.QuestionID == lastQID && len(due) > 1 {
			continue
		}
		if q, err := s.questions.GetByID(ctx, st.QuestionID); err == nil {
			return q
		}
	}
	return nil
}

// pickRandomAvoid は all から lastQID 以外を 1 件返す (要素 1 個なら lastQID を返す)。
func pickRandomAvoid(all []domain.Question, lastQID int64, r *rand.Rand) *domain.Question {
	if len(all) == 1 {
		return &all[0]
	}
	for range 10 {
		idx := r.IntN(len(all))
		if all[idx].ID != lastQID {
			return &all[idx]
		}
	}
	return &all[r.IntN(len(all))]
}

// Submit は採点して attempt と SRS 状態を保存する。
func (s *QuizService) Submit(ctx context.Context, userID int64, in domain.SubmitInput) (*domain.GradedAttempt, error) {
	q, err := s.questions.GetByID(ctx, in.QuestionID)
	if err != nil {
		return nil, err
	}
	var want domain.AnswerPayload
	if err := json.Unmarshal([]byte(q.AnswerJSON), &want); err != nil {
		return nil, fmt.Errorf("quiz submit answer json: %w", err)
	}
	correct := grading.IsCorrect(want, in.Answer)

	now := s.clock.Now()
	submitted, err := json.Marshal(in.Answer)
	if err != nil {
		return nil, fmt.Errorf("quiz submit marshal: %w", err)
	}
	att := &domain.Attempt{
		UserID:              userID,
		QuestionID:          in.QuestionID,
		IsCorrect:           correct,
		DurationMs:          in.DurationMs,
		SubmittedAnswerJSON: string(submitted),
		AnsweredAt:          now,
	}
	if in.SetCode != "" {
		if set, err := s.sets.GetByCode(ctx, in.SetCode); err == nil {
			id := set.ID
			att.SetID = &id
		}
	}
	if err := s.attempts.Create(ctx, att); err != nil {
		return nil, fmt.Errorf("quiz submit create attempt: %w", err)
	}

	state, err := s.srs.Get(ctx, userID, in.QuestionID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("quiz submit srs get: %w", err)
		}
		init := srs.NewState(now)
		init.UserID = userID
		init.QuestionID = in.QuestionID
		state = &init
	}
	g := srs.GradeFromResult(correct, in.DurationMs)
	next := srs.Next(*state, g, now)
	if err := s.srs.Upsert(ctx, &next); err != nil {
		return nil, fmt.Errorf("quiz submit srs upsert: %w", err)
	}
	return &domain.GradedAttempt{
		Attempt:     *att,
		Explanation: q.Explanation,
		NextDueAt:   next.DueAt,
	}, nil
}

// DeleteAttempt は当該 attempt を削除する。
func (s *QuizService) DeleteAttempt(ctx context.Context, userID, attemptID int64) error {
	return s.attempts.DeleteByID(ctx, userID, attemptID)
}

// DeleteAllForUser はユーザのすべての attempts と SRS 状態を削除する。
func (s *QuizService) DeleteAllForUser(ctx context.Context, userID int64) error {
	if err := s.attempts.DeleteAllForUser(ctx, userID); err != nil {
		return fmt.Errorf("quiz delete attempts: %w", err)
	}
	if err := s.srs.DeleteAllForUser(ctx, userID); err != nil {
		return fmt.Errorf("quiz delete srs: %w", err)
	}
	return nil
}

// History は最近の attempts を返す。
func (s *QuizService) History(ctx context.Context, userID int64, limit, offset int) ([]domain.Attempt, error) {
	return s.attempts.ListByUser(ctx, userID, limit, offset)
}
