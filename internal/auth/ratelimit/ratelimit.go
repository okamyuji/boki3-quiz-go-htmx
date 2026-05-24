// Package ratelimit は鍵単位のレートリミットを提供する。
//
// 3 種類の実装を含む。
//   - SlidingWindow: 鍵ごとに直近 W 秒の試行回数を制限 (DoS 対策のグローバル鍵向け)。
//   - FixedWindow: 固定 W 秒のウィンドウでカウントする (ログイン試行の連続失敗向け)。
//   - TokenBucket: バースト対応の最大 C 容量、毎秒 R 補充 (per-user API 制限向け)。
package ratelimit

import (
	"sync"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/clock"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/port"
)

// SlidingWindow は鍵ごとに直近 Window の試行回数を Max に制限する。
//
// Allow 時に対象鍵のみ古い記録を剪定し、加えて空になった鍵はマップから削除する。
// アクセスのない鍵は次回 Allow で評価された際に GC されるため goroutine 不要。
type SlidingWindow struct {
	mu     sync.Mutex
	events map[string][]time.Time
	max    int
	window time.Duration
	clock  clock.Clock
}

// NewSlidingWindow は SlidingWindow を生成する。clk が nil の場合 System を使う。
func NewSlidingWindow(maxReq int, window time.Duration, clk clock.Clock) *SlidingWindow {
	if clk == nil {
		clk = clock.System{}
	}
	return &SlidingWindow{events: make(map[string][]time.Time), max: maxReq, window: window, clock: clk}
}

var _ port.RateLimiter = (*SlidingWindow)(nil)

// Allow は試行を 1 回記録し、Window 内の試行数が Max 以下なら true を返す。
func (s *SlidingWindow) Allow(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now()
	cutoff := now.Add(-s.window)
	events := s.events[key]
	pruned := events[:0]
	for _, t := range events {
		if t.After(cutoff) {
			pruned = append(pruned, t)
		}
	}
	if len(pruned) >= s.max {
		s.events[key] = pruned
		return false
	}
	pruned = append(pruned, now)
	s.events[key] = pruned
	return true
}

// Sweep は last activity が window より古い鍵をマップから削除する。
// 別 goroutine から定期的に呼ぶ想定 (Allow のホットパスを汚さない)。
func (s *SlidingWindow) Sweep() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := s.clock.Now().Add(-s.window)
	removed := 0
	for k, evs := range s.events {
		if len(evs) == 0 || !evs[len(evs)-1].After(cutoff) {
			delete(s.events, k)
			removed++
		}
	}
	return removed
}

// FixedWindow は鍵ごとに WindowStart から Window 経過するまでカウントする。
// ログイン試行失敗のカウンタなど、明確なロックアウト UI が必要な用途向け。
type FixedWindow struct {
	mu     sync.Mutex
	state  map[string]*fwState
	max    int
	window time.Duration
	clock  clock.Clock
}

type fwState struct {
	windowStart time.Time
	count       int
}

// NewFixedWindow は FixedWindow を生成する。
func NewFixedWindow(maxReq int, window time.Duration, clk clock.Clock) *FixedWindow {
	if clk == nil {
		clk = clock.System{}
	}
	return &FixedWindow{state: make(map[string]*fwState), max: maxReq, window: window, clock: clk}
}

var _ port.RateLimiter = (*FixedWindow)(nil)

// Allow は試行をカウントし、ウィンドウ内で max を超えていなければ true を返す。
func (f *FixedWindow) Allow(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.clock.Now()
	st, ok := f.state[key]
	if !ok || now.Sub(st.windowStart) >= f.window {
		f.state[key] = &fwState{windowStart: now, count: 1}
		return true
	}
	if st.count >= f.max {
		return false
	}
	st.count++
	return true
}

// Reset は鍵のカウンタをリセットする (ログイン成功後の clear 用)。
func (f *FixedWindow) Reset(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.state, key)
}

// Sweep は windowStart が window より古い鍵を削除する。
func (f *FixedWindow) Sweep() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	cutoff := f.clock.Now().Add(-f.window)
	removed := 0
	for k, st := range f.state {
		if !st.windowStart.After(cutoff) {
			delete(f.state, k)
			removed++
		}
	}
	return removed
}

// TokenBucket は鍵ごとに容量 capacity、毎秒 refillRate 補充されるトークンバケット。
type TokenBucket struct {
	mu         sync.Mutex
	buckets    map[string]*tbState
	capacity   float64
	refillRate float64 // tokens per second
	clock      clock.Clock
}

type tbState struct {
	tokens   float64
	lastFill time.Time
}

// NewTokenBucket は TokenBucket を生成する。
func NewTokenBucket(capacity, refillRatePerSec float64, clk clock.Clock) *TokenBucket {
	if clk == nil {
		clk = clock.System{}
	}
	return &TokenBucket{buckets: make(map[string]*tbState), capacity: capacity, refillRate: refillRatePerSec, clock: clk}
}

var _ port.RateLimiter = (*TokenBucket)(nil)

// Allow は 1 トークンを消費して true を返す。トークンが無ければ false。
func (t *TokenBucket) Allow(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.clock.Now()
	st, ok := t.buckets[key]
	if !ok {
		t.buckets[key] = &tbState{tokens: t.capacity - 1, lastFill: now}
		return true
	}
	elapsed := now.Sub(st.lastFill).Seconds()
	st.tokens += elapsed * t.refillRate
	if st.tokens > t.capacity {
		st.tokens = t.capacity
	}
	st.lastFill = now
	if st.tokens < 1 {
		return false
	}
	st.tokens--
	return true
}

// Sweep は lastFill が ttl より古い (= long-idle) 鍵を削除する。
func (t *TokenBucket) Sweep(ttl time.Duration) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := t.clock.Now().Add(-ttl)
	removed := 0
	for k, st := range t.buckets {
		if !st.lastFill.After(cutoff) {
			delete(t.buckets, k)
			removed++
		}
	}
	return removed
}
