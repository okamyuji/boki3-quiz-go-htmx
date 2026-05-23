# boki3-quiz-go-htmx Phase 3: SQLite Repository + Migrations 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `?` プレースホルダのみを使う SQLite Repository 実装と embed.FS によるマイグレーション機構を完成させる。全 Repository は `internal/port` の interface を満たし、一時 sqlite ファイルでの統合テストで挙動を担保する。

**Architecture:** `modernc.org/sqlite` を `database/sql` ドライバとして利用。マイグレーションは `migrations/*.sql` を `embed.FS` で読み込み、`schema_migrations` テーブルで適用済みを追跡する単純な up-only マイグレータ。各 Repository は単一テーブルを責務とし、複数テーブル横断はサービス層で組み立てる。

**Tech Stack:** Go 1.25 / database/sql / modernc.org/sqlite v1.36+ / embed / testing

**Spec:** `docs/superpowers/specs/2026-05-24-boki3-quiz-design.md` の section 5 と 9.4。
**Roadmap:** `docs/superpowers/plans/2026-05-24-boki3-quiz-roadmap.md`

---

## File Structure (Phase 3 終了時)

```
internal/
  pkg/
    sqlitex/                       database/sql + sqlite open ヘルパ
      sqlitex.go
      sqlitex_test.go
    clock/
      clock.go                     time.Now() の interface 化
  port/
    repo.go                        Repository interface 群
    util.go                        Clock, IDGenerator interface
  repo/sqlite/
    migrations.go                  embed + マイグレータ
    migrations_test.go
    user_repo.go
    user_repo_test.go
    session_repo.go
    session_repo_test.go
    jwt_repo.go
    jwt_repo_test.go
    question_repo.go
    question_repo_test.go
    set_repo.go
    set_repo_test.go
    attempt_repo.go
    attempt_repo_test.go
    srs_repo.go
    srs_repo_test.go
    testdb.go                       テスト共通の一時 DB ヘルパ
migrations/
  0001_init.sql                    最初の up マイグレーション
```

各 Repository は単一テーブル責務。`testdb.go` は `t.TempDir()` 配下に一意 DB を作り、マイグレーションを適用し、`*sql.DB` と cleanup を返す。

---

### Task 1: modernc.org/sqlite を依存に追加

- [ ] **Step 1: get と go mod tidy**

```bash
cd /Users/yujiokamoto/devs/golang/boki3-quiz-go-htmx
go get modernc.org/sqlite@latest
go mod tidy
```

- [ ] **Step 2: go.sum と go.mod のコミット**

```bash
git add go.mod go.sum
git -c commit.gpgsign=false commit -m "chore(deps): add modernc.org/sqlite (pure-go sqlite driver)"
```

---

### Task 2: `internal/pkg/clock` (Clock interface)

- [ ] **Step 1: ファイル作成**

`internal/pkg/clock/clock.go`:
```go
// Package clock abstracts time.Now() so that tests can inject a fake clock.
package clock

import "time"

// Clock returns the current time. The default implementation calls time.Now in UTC.
type Clock interface {
	Now() time.Time
}

// System is a Clock backed by time.Now().UTC().
type System struct{}

// Now returns time.Now().UTC().
func (System) Now() time.Time { return time.Now().UTC() }

// Fixed is a Clock that always returns t (useful in tests).
type Fixed struct{ T time.Time }

// Now returns the fixed time.
func (f Fixed) Now() time.Time { return f.T }
```

- [ ] **Step 2: コミット**

```bash
git add internal/pkg/clock/
git -c commit.gpgsign=false commit -m "feat(pkg/clock): introduce Clock interface and fixtures"
```

---

### Task 3: `internal/port` interface 群

- [ ] **Step 1: util.go**

`internal/port/util.go`:
```go
// Package port defines all repository and service interfaces used by service and transport layers.
package port

import (
	"context"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// IDGenerator はトークンと UUID を生成するためのインターフェース。
type IDGenerator interface {
	NewToken(bytes int) (string, error) // n バイトの CSPRNG を hex 文字列で返す
	NewUUID() string                    // ランダム UUIDv4
}

// PasswordHasher はパスワードのハッシュ生成と検証を行う。
type PasswordHasher interface {
	Hash(plain string) (hash, salt []byte, params string, err error)
	Verify(plain string, hash, salt []byte, params string) (bool, error)
}

// JWTSigner は HS256 JWT の発行と検証を行う。
type JWTSigner interface {
	Sign(claims domain.JWTClaims) (string, error)
	Parse(token string) (domain.JWTClaims, error)
}

// RateLimiter は鍵単位での許容判定を行う。
type RateLimiter interface {
	Allow(key string) bool
}

// Unused import guard (context, time are used below in repo.go via same package).
var _ = context.TODO
var _ = time.Now
```

- [ ] **Step 2: repo.go**

`internal/port/repo.go`:
```go
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
	ListBySet(ctx context.Context, setCode string) ([]domain.Question, error)
	Search(ctx context.Context, filter domain.QuestionFilter) ([]domain.Question, error)
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
}

// SRSStateRepository は srs_states テーブルへの操作を提供する。
type SRSStateRepository interface {
	Upsert(ctx context.Context, s *srs.State) error
	DueForUser(ctx context.Context, userID int64, now time.Time, limit int) ([]srs.State, error)
	Get(ctx context.Context, userID, questionID int64) (*srs.State, error)
	DeleteAllForUser(ctx context.Context, userID int64) error
	CountDueForUser(ctx context.Context, userID int64, now time.Time) (int, error)
}
```

- [ ] **Step 3: ビルドとコミット**

```bash
rm -f internal/port/.keep
go build ./internal/port/...
git add internal/port/
git -c commit.gpgsign=false commit -m "feat(port): define repository and utility interfaces"
```

---

### Task 4: 最初のマイグレーション SQL

`migrations/0001_init.sql` を Phase 2 の設計書 5.2 のとおりに作成。WAL モードや busy_timeout は接続側 PRAGMA で設定する。

(以下、SQL 全文は設計書 5.2 と一致。Task 内で完全 copy)

```sql
-- 0001_init.sql
-- 初回スキーマ。users / sessions / jwt_revocations / topics / questions / question_sets /
-- question_set_members / attempts / srs_states を作成する。

CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  username TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash BLOB NOT NULL,
  password_salt BLOB NOT NULL,
  password_params TEXT NOT NULL,
  password_updated_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  csrf_token TEXT NOT NULL,
  user_agent TEXT,
  ip TEXT,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  last_seen_at INTEGER NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions(user_id);

CREATE TABLE jwt_revocations (
  jti TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL,
  revoked_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX idx_jwt_rev_user ON jwt_revocations(user_id);

CREATE TABLE topics (
  id INTEGER PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  ord INTEGER NOT NULL
);

CREATE TABLE questions (
  id INTEGER PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  topic_id INTEGER NOT NULL REFERENCES topics(id),
  question_type TEXT NOT NULL,
  difficulty INTEGER NOT NULL,
  prompt TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  answer_json TEXT NOT NULL,
  explanation TEXT NOT NULL,
  references_json TEXT,
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_questions_topic ON questions(topic_id);
CREATE INDEX idx_questions_type ON questions(question_type);

CREATE TABLE question_sets (
  id INTEGER PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  target_size INTEGER NOT NULL
);

CREATE TABLE question_set_members (
  set_id INTEGER NOT NULL REFERENCES question_sets(id) ON DELETE CASCADE,
  question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  ord INTEGER NOT NULL,
  PRIMARY KEY(set_id, question_id)
);

CREATE TABLE attempts (
  id INTEGER PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  set_id INTEGER REFERENCES question_sets(id) ON DELETE SET NULL,
  is_correct INTEGER NOT NULL,
  duration_ms INTEGER NOT NULL,
  submitted_answer_json TEXT NOT NULL,
  answered_at INTEGER NOT NULL
);
CREATE INDEX idx_attempts_user_time ON attempts(user_id, answered_at);
CREATE INDEX idx_attempts_user_q ON attempts(user_id, question_id);

CREATE TABLE srs_states (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  efactor REAL NOT NULL,
  interval_days INTEGER NOT NULL,
  repetitions INTEGER NOT NULL,
  due_at INTEGER NOT NULL,
  last_grade INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY(user_id, question_id)
);
CREATE INDEX idx_srs_due ON srs_states(user_id, due_at);
```

---

### Task 5: `internal/pkg/sqlitex` (Open + Pragma + Apply)

- 標準 PRAGMA: `journal_mode=WAL`, `foreign_keys=ON`, `busy_timeout=5000`, `synchronous=NORMAL`
- `embed.FS` から `.sql` を順次 Apply
- `schema_migrations(version INTEGER PRIMARY KEY, applied_at INTEGER)` テーブルで重複適用を防止
- テスト: 1) 適用前に schema_migrations 0 行 2) 適用後 0001 が記録 3) 二度目の Apply は no-op

(コードは Task 7 と同形なのでこの計画書では概要のみ。実装時に各 Repository テストと並行で書く)

---

### Task 6〜13: 各 Repository 実装 (TDD)

各 Repository は同形のステップで TDD します。

各 Repository の責務:

| Repository | テーブル | 主メソッド |
|---|---|---|
| user_repo | users | Create / FindByUsername / FindByID / UpdatePassword |
| session_repo | sessions | Create / FindByID / Touch / Delete / DeleteAllForUser / DeleteAllForUserExcept / PurgeExpired |
| jwt_repo | jwt_revocations | Revoke / IsRevoked / RevokeAllForUser / PurgeExpired |
| question_repo | questions | GetByID / ListBySet (JOIN question_set_members) / Search |
| set_repo | question_sets | GetByCode / ListAll |
| attempt_repo | attempts | Create / ListByUser / DeleteByID / DeleteAllForUser / StatsByTopic (JOIN topics) / DailyAccuracy / SummaryForUser |
| srs_repo | srs_states | Upsert / DueForUser / Get / DeleteAllForUser / CountDueForUser |

各テストは以下 5 ステップを 1 タスク (15 分単位) として実行します。

1. テストを書く (RED): `t.TempDir()` で一時 DB を作り、テスト対象メソッドを呼び期待値検証
2. 失敗確認: `go test ./internal/repo/sqlite/... -run TestName`
3. 実装: 単一ファイルに `?` プレースホルダのみで実装
4. 成功確認: `go test -count=1 -race ./internal/repo/sqlite/...`
5. コミット

---

### Task 14: Phase 3 全体動作確認

```bash
bash scripts/quality-gate.sh
```

すべて Pass し、カバレッジが repo 層で目安 80% を超えていること。

## Phase 3 完了条件

1. すべての Repository interface に sqlite 実装が存在する
2. マイグレータが embed.FS から `0001_init.sql` を適用し idempotent
3. 全テストが `count=1 shuffle=on race` で Pass
4. `bash scripts/quality-gate.sh` 全 Pass

完了後、Phase 4 (Service 層 + 認証 + JWT 自作) の計画書作成へ進む。

---

## 補足: 各 Repository のコード雛形

Phase 3 実装時に参照する SQL の例を残します。

### users

```go
const insertUserSQL = `INSERT INTO users(
  username, password_hash, password_salt, password_params, password_updated_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`

const selectUserByUsernameSQL = `SELECT id, username, password_hash, password_salt, password_params,
  password_updated_at, created_at, updated_at FROM users WHERE username = ? COLLATE NOCASE`
```

### sessions の DeleteAllForUserExcept

```go
const deleteOtherSessionsSQL = `DELETE FROM sessions WHERE user_id = ? AND id != ?`
```

### attempts の DailyAccuracy

now を YYYY-MM-DD UTC で切り、`answered_at >= ?` で過去 N 日に絞った後、SQLite の `strftime('%Y-%m-%d', answered_at, 'unixepoch')` で日次 GROUP BY する。

### srs_states の Upsert

```go
const upsertSRSSQL = `INSERT INTO srs_states(
  user_id, question_id, efactor, interval_days, repetitions, due_at, last_grade, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id, question_id) DO UPDATE SET
  efactor=excluded.efactor,
  interval_days=excluded.interval_days,
  repetitions=excluded.repetitions,
  due_at=excluded.due_at,
  last_grade=excluded.last_grade,
  updated_at=excluded.updated_at`
```

すべて `?` プレースホルダのみで `fmt.Sprintf` での値埋め込みは禁止 (`check_no_sql_sprintf.sh` でガード)。
