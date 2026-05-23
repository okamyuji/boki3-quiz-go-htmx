package domain_test

import (
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

func TestTopicStatAccuracyZeroWhenEmpty(t *testing.T) {
	t.Parallel()
	s := domain.TopicStat{Total: 0, Correct: 0}
	if got := s.Accuracy(); got != 0 {
		t.Fatalf("Accuracy() = %f, want 0", got)
	}
}

func TestTopicStatAccuracyComputes(t *testing.T) {
	t.Parallel()
	s := domain.TopicStat{Total: 4, Correct: 3}
	if got := s.Accuracy(); got != 0.75 {
		t.Fatalf("Accuracy() = %f, want 0.75", got)
	}
}

func TestSessionIsExpired(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	s := domain.Session{ExpiresAt: now}
	if !s.IsExpired(now) {
		t.Fatalf("IsExpired(equal) = false, want true")
	}
	if !s.IsExpired(now.Add(time.Second)) {
		t.Fatalf("IsExpired(after) = false, want true")
	}
	if s.IsExpired(now.Add(-time.Second)) {
		t.Fatalf("IsExpired(before) = true, want false")
	}
}

func TestDefaultSetCodesContainsAllThree(t *testing.T) {
	t.Parallel()
	codes := domain.DefaultSetCodes()
	if len(codes) != 3 {
		t.Fatalf("DefaultSetCodes len = %d, want 3", len(codes))
	}
	wantSet := map[string]bool{
		domain.SetCodeCore:          true,
		domain.SetCodeJournal:       true,
		domain.SetCodeComprehensive: true,
	}
	for _, c := range codes {
		if !wantSet[c] {
			t.Fatalf("unexpected code %q", c)
		}
	}
}
