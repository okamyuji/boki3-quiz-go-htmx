package ratelimit_test

import (
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/ratelimit"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/clock"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }
func (f *fakeClock) advance(d time.Duration) {
	f.t = f.t.Add(d)
}

func TestSlidingWindowAllowsUpToMax(t *testing.T) {
	t.Parallel()
	clk := &fakeClock{t: time.Unix(1000, 0)}
	rl := ratelimit.NewSlidingWindow(3, time.Second, clk)
	for i := range 3 {
		if !rl.Allow("k") {
			t.Fatalf("Allow %d = false", i)
		}
	}
	if rl.Allow("k") {
		t.Fatalf("4th Allow should fail")
	}
}

func TestSlidingWindowReleasesAfterWindow(t *testing.T) {
	t.Parallel()
	clk := &fakeClock{t: time.Unix(1000, 0)}
	rl := ratelimit.NewSlidingWindow(2, time.Second, clk)
	rl.Allow("k")
	rl.Allow("k")
	if rl.Allow("k") {
		t.Fatalf("3rd should fail")
	}
	clk.advance(2 * time.Second)
	if !rl.Allow("k") {
		t.Fatalf("after window expired, should succeed")
	}
}

func TestFixedWindowAndReset(t *testing.T) {
	t.Parallel()
	clk := &fakeClock{t: time.Unix(1000, 0)}
	rl := ratelimit.NewFixedWindow(2, time.Minute, clk)
	if !rl.Allow("u") {
		t.Fatalf("first should succeed")
	}
	if !rl.Allow("u") {
		t.Fatalf("second should succeed")
	}
	if rl.Allow("u") {
		t.Fatalf("3rd should fail")
	}
	rl.Reset("u")
	if !rl.Allow("u") {
		t.Fatalf("after reset should succeed")
	}
}

func TestTokenBucketBurstAndRefill(t *testing.T) {
	t.Parallel()
	clk := &fakeClock{t: time.Unix(1000, 0)}
	tb := ratelimit.NewTokenBucket(5, 1, clk) // 5 burst, 1 token/sec
	for i := range 5 {
		if !tb.Allow("u") {
			t.Fatalf("burst %d failed", i)
		}
	}
	if tb.Allow("u") {
		t.Fatalf("after burst should fail")
	}
	clk.advance(2 * time.Second)
	if !tb.Allow("u") {
		t.Fatalf("after refill should succeed")
	}
}

func TestNilClockDefaultsToSystem(t *testing.T) {
	t.Parallel()
	rl := ratelimit.NewSlidingWindow(1, time.Minute, nil)
	if !rl.Allow("k") {
		t.Fatalf("default clock initial Allow should succeed")
	}
	// 2 回目はブロックされるが、ここでは clock.System のため確認のみ。
	var _ = clock.System{}
}
