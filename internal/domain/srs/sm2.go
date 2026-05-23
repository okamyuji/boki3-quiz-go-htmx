package srs

import (
	"math"
	"time"
)

// Next は SM-2 風の次状態を算出する純関数。
//   - g < 3 のとき Repetitions=0, IntervalDays=1 へリセット
//   - g >= 3 のとき Repetitions に応じて 1, 6, prev*EFactor を IntervalDays に設定
//   - EFactor は最低 1.3 にクランプ
func Next(s State, g Grade, now time.Time) State {
	if g < GradeSlow {
		s.Repetitions = 0
		s.IntervalDays = 1
	} else {
		switch s.Repetitions {
		case 0:
			s.IntervalDays = 1
		case 1:
			s.IntervalDays = 6
		default:
			s.IntervalDays = int(math.Round(float64(s.IntervalDays) * s.EFactor))
		}
		s.Repetitions++
	}
	delta := 0.1 - float64(5-g)*(0.08+float64(5-g)*0.02)
	if next := s.EFactor + delta; next > 1.3 {
		s.EFactor = next
	} else {
		s.EFactor = 1.3
	}
	s.DueAt = now.AddDate(0, 0, s.IntervalDays)
	s.LastGrade = g
	s.UpdatedAt = now
	return s
}
