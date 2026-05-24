package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"

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
		rng: func() *rand.Rand { return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) },
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
		return &all[0], nil
	case domain.QuizModeSRS, "":
		return s.pickSRS(ctx, userID, all)
	default:
		return nil, domain.ErrInvalidInput
	}
}

// pickSRS は学習モード SRS の選定ロジック。
//
// 既定重み: due 70% / 不正解履歴 20% / ランダム 10%。
// due が空の場合は残り 30% を 20:10 比で再分配 (不正解 66.6% / ランダム 33.3%)。
// 履歴フォールバックが空ならランダムに落ちる。
func (s *QuizService) pickSRS(ctx context.Context, userID int64, all []domain.Question) (*domain.Question, error) {
	now := s.clock.Now()
	due, err := s.srs.DueForUser(ctx, userID, now, 50)
	if err != nil {
		return nil, fmt.Errorf("quiz srs due: %w", err)
	}
	r := s.rng()
	if len(due) > 0 && r.Float64() < 0.7 {
		q, err := s.questions.GetByID(ctx, due[r.IntN(len(due))].QuestionID)
		if err == nil {
			return q, nil
		}
	}
	// 残り母数 (due が空なら 100%、そうでなければ 30%) を 2:1 で分割。
	if r.Float64() < 2.0/3.0 {
		attempts, err := s.attempts.ListByUser(ctx, userID, 50, 0)
		if err == nil {
			for _, a := range attempts {
				if !a.IsCorrect {
					q, err := s.questions.GetByID(ctx, a.QuestionID)
					if err == nil {
						return q, nil
					}
				}
			}
		}
	}
	return &all[r.IntN(len(all))], nil
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
		IsCorrect:   correct,
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
