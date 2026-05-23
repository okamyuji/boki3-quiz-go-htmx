package port

import (
	"context"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// AuthService は登録/ログイン/パスワード変更/ログアウトを提供する。
type AuthService interface {
	Register(ctx context.Context, username, password string) (*domain.User, error)
	Login(ctx context.Context, username, password, userAgent, ip string) (*domain.Session, error)
	Logout(ctx context.Context, sessionID string) error
	ChangePassword(ctx context.Context, userID int64, currentPlain, newPlain, keepSessionID string) error
	SessionByID(ctx context.Context, sessionID string) (*domain.Session, *domain.User, error)
}

// APIAuthService は JWT 発行と検証を提供する。
type APIAuthService interface {
	IssueToken(ctx context.Context, userID int64, ttl time.Duration) (token string, jti string, expiresAt time.Time, err error)
	VerifyToken(ctx context.Context, token string) (int64, error)
	Revoke(ctx context.Context, jti string, userID int64, expiresAt time.Time) error
}

// QuizService は学習セッション (出題・採点・履歴管理) を提供する。
type QuizService interface {
	NextQuestion(ctx context.Context, userID int64, setCode string, mode domain.QuizMode) (*domain.Question, error)
	Submit(ctx context.Context, userID int64, in domain.SubmitInput) (*domain.GradedAttempt, error)
	DeleteAttempt(ctx context.Context, userID, attemptID int64) error
	DeleteAllForUser(ctx context.Context, userID int64) error
	History(ctx context.Context, userID int64, limit, offset int) ([]domain.Attempt, error)
}

// StatsService はホーム画面と進捗ページの集計値を返す。
type StatsService interface {
	Summary(ctx context.Context, userID int64) (domain.StatsSummary, error)
	TopicStats(ctx context.Context, userID int64) ([]domain.TopicStat, error)
	DailyAccuracy(ctx context.Context, userID int64, days int) ([]domain.DailyAccuracy, error)
}
