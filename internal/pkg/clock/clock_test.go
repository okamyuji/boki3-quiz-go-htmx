package clock_test

import (
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/clock"
)

func TestSystemNowIsUTC(t *testing.T) {
	t.Parallel()
	got := clock.System{}.Now()
	if got.Location() != time.UTC {
		t.Fatalf("System.Now() Location = %v, want UTC", got.Location())
	}
}

func TestFixedNow(t *testing.T) {
	t.Parallel()
	want := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	got := clock.Fixed{T: want}.Now()
	if !got.Equal(want) {
		t.Fatalf("Fixed.Now() = %v, want %v", got, want)
	}
}
