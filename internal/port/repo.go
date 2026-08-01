package port

import (
	"context"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/srs"
)

// UserRepository は users テーブルへの操作を提供する。
type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	FindByID(ctx context.Context, id int64) (*domain.User, error)
	UpdatePassword(ctx context.Context, id int64, hash, salt []byte, params string, at time.Time) error
}

// SessionRepository は sessions テーブルへの操作を提供する。
type SessionRepository interface {
	Create(ctx context.Context, s *domain.Session) error
	FindByID(ctx context.Context, id string) (*domain.Session, error)
	Touch(ctx context.Context, id string, lastSeen time.Time) error
	Delete(ctx context.Context, id string) error
	DeleteAllForUser(ctx context.Context, userID int64) error
	DeleteAllForUserExcept(ctx context.Context, userID int64, keepID string) error
	PurgeExpired(ctx context.Context, now time.Time) (int, error)
}

// JWTRevocationRepository は jwt_revocations テーブルへの操作を提供する。
type JWTRevocationRepository interface {
	Revoke(ctx context.Context, jti string, userID int64, expiresAt time.Time) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
	RevokeAllForUser(ctx context.Context, userID int64, now time.Time) error
	PurgeExpired(ctx context.Context, now time.Time) (int, error)
}

// QuestionRepository は questions テーブルへの読取操作を提供する。
type QuestionRepository interface {
	GetByID(ctx context.Context, id int64) (*domain.Question, error)
	GetByCode(ctx context.Context, code string) (*domain.Question, error)
	ListBySet(ctx context.Context, setCode string) ([]domain.Question, error)
	Search(ctx context.Context, filter domain.QuestionFilter) ([]domain.Question, error)
}

// TopicRepository は topics テーブルへの読取操作を提供する。
type TopicRepository interface {
	ListAll(ctx context.Context) ([]domain.Topic, error)
}

// SetRepository は question_sets / question_set_members への操作を提供する。
type SetRepository interface {
	GetByCode(ctx context.Context, code string) (*domain.QuestionSet, error)
	ListAll(ctx context.Context) ([]domain.QuestionSet, error)
}

// AttemptRepository は attempts テーブルへの操作と集計を提供する。
type AttemptRepository interface {
	Create(ctx context.Context, a *domain.Attempt) error
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.Attempt, error)
	DeleteByID(ctx context.Context, userID, attemptID int64) error
	DeleteAllForUser(ctx context.Context, userID int64) error
	StatsByTopic(ctx context.Context, userID int64) ([]domain.TopicStat, error)
	DailyAccuracy(ctx context.Context, userID int64, days int, now time.Time) ([]domain.DailyAccuracy, error)
	SummaryForUser(ctx context.Context, userID int64) (totalAttempts, totalCorrect int, err error)
	// LastQuestionIDInSet は当該セット内でユーザが最後に回答した問題 ID を返す。
	// 回答がなければ domain.ErrNotFound。
	LastQuestionIDInSet(ctx context.Context, userID, setID int64) (int64, error)
	// WeakTopicIDs は since 以降の回答を論点別に集計し、誤答を含む論点を誤答率降順で返す。
	WeakTopicIDs(ctx context.Context, userID int64, since time.Time, limit int) ([]int64, error)
	// AttemptedQuestionIDs はユーザが 1 回以上回答した問題 ID を昇順で返す。
	AttemptedQuestionIDs(ctx context.Context, userID int64) ([]int64, error)
}

// UserPrefsRepository は user_prefs テーブルへの操作を提供する。
type UserPrefsRepository interface {
	Get(ctx context.Context, userID int64) (*domain.UserPrefs, error)
	Upsert(ctx context.Context, p *domain.UserPrefs) error
}

// SRSStateRepository は srs_states テーブルへの操作を提供する。
type SRSStateRepository interface {
	Upsert(ctx context.Context, s *srs.State) error
	DueForUser(ctx context.Context, userID int64, now time.Time, limit int) ([]srs.State, error)
	Get(ctx context.Context, userID, questionID int64) (*srs.State, error)
	DeleteAllForUser(ctx context.Context, userID int64) error
	CountDueForUser(ctx context.Context, userID int64, now time.Time) (int, error)
}
