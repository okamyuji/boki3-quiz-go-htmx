package srs_test

import (
	"math"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/srs"
)

func TestInitialState(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)
	s := srs.NewState(now)
	if s.EFactor != 2.5 {
		t.Fatalf("EFactor = %f, want 2.5", s.EFactor)
	}
	if s.Repetitions != 0 {
		t.Fatalf("Repetitions = %d, want 0", s.Repetitions)
	}
	if !s.DueAt.Equal(now) {
		t.Fatalf("DueAt = %v, want %v", s.DueAt, now)
	}
}

func TestNextOnPerfect(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)
	s := srs.NewState(now)

	s1 := srs.Next(s, srs.GradePerfect, now)
	if s1.IntervalDays != 1 {
		t.Fatalf("first interval = %d, want 1", s1.IntervalDays)
	}
	if s1.Repetitions != 1 {
		t.Fatalf("repetitions after first = %d, want 1", s1.Repetitions)
	}
	if !s1.DueAt.Equal(now.AddDate(0, 0, 1)) {
		t.Fatalf("due = %v, want %v", s1.DueAt, now.AddDate(0, 0, 1))
	}

	s2 := srs.Next(s1, srs.GradePerfect, now.AddDate(0, 0, 1))
	if s2.IntervalDays != 6 {
		t.Fatalf("second interval = %d, want 6", s2.IntervalDays)
	}

	s3 := srs.Next(s2, srs.GradePerfect, now.AddDate(0, 0, 7))
	want3 := int(math.Round(6 * s2.EFactor))
	if s3.IntervalDays != want3 {
		t.Fatalf("third interval = %d, want %d", s3.IntervalDays, want3)
	}
}

func TestNextOnFailResetsRepetition(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)
	s := srs.NewState(now)
	s = srs.Next(s, srs.GradePerfect, now)
	s = srs.Next(s, srs.GradePerfect, now.AddDate(0, 0, 1))
	if s.Repetitions != 2 {
		t.Fatalf("setup: Repetitions = %d, want 2", s.Repetitions)
	}

	s = srs.Next(s, srs.GradeWorst, now.AddDate(0, 0, 7))
	if s.Repetitions != 0 {
		t.Fatalf("Repetitions after fail = %d, want 0", s.Repetitions)
	}
	if s.IntervalDays != 1 {
		t.Fatalf("IntervalDays after fail = %d, want 1", s.IntervalDays)
	}
}

func TestEFactorClampedAtMin(t *testing.T) {
	t.Parallel()
	now := time.Now()
	s := srs.NewState(now)
	for i := range 30 {
		s = srs.Next(s, srs.GradeWorst, now.AddDate(0, 0, i))
	}
	if s.EFactor < 1.3 {
		t.Fatalf("EFactor = %f, want >= 1.3", s.EFactor)
	}
}
