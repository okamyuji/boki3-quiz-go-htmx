package domain

import "time"

// Attempt は回答試行 1 件。submitted_answer_json は JSON 文字列のまま保持する。
type Attempt struct {
	ID                  int64
	UserID              int64
	QuestionID          int64
	SetID               *int64 // null の場合あり
	IsCorrect           bool
	DurationMs          int
	SubmittedAnswerJSON string
	AnsweredAt          time.Time
}

// TopicStat は論点別の正解率統計。
type TopicStat struct {
	TopicCode string
	TopicName string
	Total     int
	Correct   int
}

// Accuracy は 0.0〜1.0 の正解率を返す。Total=0 のとき 0.0。
func (s TopicStat) Accuracy() float64 {
	if s.Total == 0 {
		return 0
	}
	return float64(s.Correct) / float64(s.Total)
}

// DailyAccuracy は 1 日単位の正解率。
type DailyAccuracy struct {
	Date    time.Time // 日付の 00:00:00 UTC
	Total   int
	Correct int
}

// StatsSummary はホーム画面用のサマリ。
type StatsSummary struct {
	TotalAttempts int
	TotalCorrect  int
	OverallRate   float64
	DueCount      int
}

// GradedAttempt は Submit の戻り。
// IsCorrect は Attempt.IsCorrect で参照する (重複保持しない)。
type GradedAttempt struct {
	Attempt     Attempt
	Explanation string
	NextDueAt   time.Time
}

// IsCorrect は埋め込み Attempt の IsCorrect を返すショートカット。
func (g GradedAttempt) IsCorrect() bool { return g.Attempt.IsCorrect }
