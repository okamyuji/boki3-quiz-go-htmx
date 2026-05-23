// Package clock は time.Now() を抽象化し、テストでの差し替えを可能にする。
package clock

import "time"

// Clock は現在時刻を返す。実装は通常 time.Now().UTC() を返す。
type Clock interface {
	Now() time.Time
}

// System は time.Now().UTC() を返す本番用 Clock。
type System struct{}

// Now は現在 UTC 時刻を返す。
func (System) Now() time.Time { return time.Now().UTC() }

// Fixed は固定時刻を返すテスト用 Clock。
type Fixed struct{ T time.Time }

// Now は固定時刻を返す。
func (f Fixed) Now() time.Time { return f.T }
