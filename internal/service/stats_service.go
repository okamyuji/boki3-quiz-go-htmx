package service

import (
	"context"
	"fmt"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/clock"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

// StatsService は port.StatsService の実装。
type StatsService struct {
	attempts port.AttemptRepository
	srs      port.SRSStateRepository
	clock    clock.Clock
}

// NewStatsService は StatsService を生成する。
func NewStatsService(a port.AttemptRepository, s port.SRSStateRepository, clk clock.Clock) *StatsService {
	if clk == nil {
		clk = clock.System{}
	}
	return &StatsService{attempts: a, srs: s, clock: clk}
}

var _ port.StatsService = (*StatsService)(nil)

// Summary はホーム画面用のサマリを返す。
func (s *StatsService) Summary(ctx context.Context, userID int64) (domain.StatsSummary, error) {
	total, correct, err := s.attempts.SummaryForUser(ctx, userID)
	if err != nil {
		return domain.StatsSummary{}, fmt.Errorf("stats summary attempts: %w", err)
	}
	due, err := s.srs.CountDueForUser(ctx, userID, s.clock.Now())
	if err != nil {
		return domain.StatsSummary{}, fmt.Errorf("stats summary due: %w", err)
	}
	var rate float64
	if total > 0 {
		rate = float64(correct) / float64(total)
	}
	return domain.StatsSummary{
		TotalAttempts: total,
		TotalCorrect:  correct,
		OverallRate:   rate,
		DueCount:      due,
	}, nil
}

// TopicStats は論点別の集計を返す。
func (s *StatsService) TopicStats(ctx context.Context, userID int64) ([]domain.TopicStat, error) {
	return s.attempts.StatsByTopic(ctx, userID)
}

// DailyAccuracy は過去 days 日の日次正解率を返す。
func (s *StatsService) DailyAccuracy(ctx context.Context, userID int64, days int) ([]domain.DailyAccuracy, error) {
	return s.attempts.DailyAccuracy(ctx, userID, days, s.clock.Now())
}
