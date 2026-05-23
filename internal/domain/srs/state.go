package srs

import "time"

// State は (user, question) に対する SRS 状態。
type State struct {
	UserID       int64
	QuestionID   int64
	EFactor      float64
	IntervalDays int
	Repetitions  int
	DueAt        time.Time
	LastGrade    Grade
	UpdatedAt    time.Time
}

// NewState は新規ユーザー/問題ペアの初期状態を返す。今すぐ due。
func NewState(now time.Time) State {
	return State{
		EFactor:      2.5,
		IntervalDays: 0,
		Repetitions:  0,
		DueAt:        now,
		LastGrade:    GradeWorst,
		UpdatedAt:    now,
	}
}
