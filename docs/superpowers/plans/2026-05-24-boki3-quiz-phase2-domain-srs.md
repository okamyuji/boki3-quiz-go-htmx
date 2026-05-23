# boki3-quiz-go-htmx Phase 2: Domain + SRS 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 副作用ゼロのドメイン層 (`internal/domain/**`) を TDD で実装し、SM-2 風 SRS、採点関数、Grade マッピング、ID/フィルタ等の型を確定する。

**Architecture:** すべて純粋関数 + 値オブジェクト。`time.Time` などの副作用は呼び出し側から受け取り、`internal/domain` は他層に依存しない。

**Tech Stack:** Go 1.25 標準ライブラリのみ。`math` の他は依存ゼロ。

**Spec:** `docs/superpowers/specs/2026-05-24-boki3-quiz-design.md` の section 5、10、11.3。
**Roadmap:** `docs/superpowers/plans/2026-05-24-boki3-quiz-roadmap.md`

---

## File Structure (Phase 2 終了時)

```
internal/domain/
├── user.go                ユーザー値オブジェクト
├── session.go             セッション値オブジェクト
├── topic.go               論点 (固定 15 種)
├── question.go            問題・回答ペイロード・フィルタ
├── question_test.go
├── attempt.go             回答試行・統計 DTO
├── set.go                 問題セット
├── srs/
│   ├── grade.go           Grade enum と Map
│   ├── grade_test.go
│   ├── state.go           SRS 状態
│   ├── sm2.go             SM-2 風次状態算出
│   └── sm2_test.go
├── grading/
│   ├── grading.go         仕訳採点
│   └── grading_test.go
├── jwt.go                 JWT クレーム (HS256)
└── errors.go              ドメイン共通エラー
```

各ファイルが単一責務になるよう分割します。

---

### Task 1: ドメイン共通エラー定義

**Files:**
- Create: `internal/domain/errors.go`

- [ ] **Step 1: ファイル作成**

`internal/domain/errors.go`:
```go
// Package domain holds value objects and pure business logic.
// No package under internal/domain may import other internal packages.
package domain

import "errors"

// Sentinel errors are exposed so service/transport layers can map to HTTP statuses.
var (
	ErrNotFound          = errors.New("domain: not found")
	ErrAlreadyExists     = errors.New("domain: already exists")
	ErrInvalidInput      = errors.New("domain: invalid input")
	ErrUnauthorized      = errors.New("domain: unauthorized")
	ErrForbidden         = errors.New("domain: forbidden")
	ErrPasswordMismatch  = errors.New("domain: password mismatch")
	ErrPasswordTooWeak   = errors.New("domain: password too weak")
	ErrUsernameInvalid   = errors.New("domain: username invalid")
	ErrRateLimited       = errors.New("domain: rate limited")
	ErrSessionExpired    = errors.New("domain: session expired")
	ErrTokenInvalid      = errors.New("domain: token invalid")
	ErrTokenExpired      = errors.New("domain: token expired")
	ErrIntegrityMismatch = errors.New("domain: integrity mismatch")
)
```

- [ ] **Step 2: ビルド確認**

```bash
go build ./internal/domain/...
```

Expected: ビルド成功。

- [ ] **Step 3: コミット**

```bash
git add internal/domain/errors.go
git -c commit.gpgsign=false commit -m "feat(domain): introduce sentinel errors"
```

---

### Task 2: User / Session 値オブジェクト

**Files:**
- Create: `internal/domain/user.go`
- Create: `internal/domain/session.go`

- [ ] **Step 1: user.go 作成**

```go
package domain

import "time"

// User は永続化される利用者表現で、パスワードハッシュは raw bytes を保持する。
type User struct {
	ID                int64
	Username          string
	PasswordHash      []byte
	PasswordSalt      []byte
	PasswordParams    string
	PasswordUpdatedAt time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
```

- [ ] **Step 2: session.go 作成**

```go
package domain

import "time"

// Session は Web UI 用 Cookie セッションを表す。CSRFToken はダブルサブミット用。
type Session struct {
	ID         string
	UserID     int64
	CSRFToken  string
	UserAgent  string
	IP         string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastSeenAt time.Time
}

// IsExpired は now との比較で有効期限切れか判定する。
func (s Session) IsExpired(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}
```

- [ ] **Step 3: ビルド確認とコミット**

```bash
go build ./internal/domain/...
git add internal/domain/user.go internal/domain/session.go
git -c commit.gpgsign=false commit -m "feat(domain): add User and Session value objects"
```

---

### Task 3: Topic 定数

**Files:**
- Create: `internal/domain/topic.go`
- Test: `internal/domain/topic_test.go`

- [ ] **Step 1: テストを書く (RED)**

`internal/domain/topic_test.go`:
```go
package domain_test

import (
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

func TestTopicsCount(t *testing.T) {
	t.Parallel()
	if got, want := len(domain.Topics()), 15; got != want {
		t.Fatalf("len(Topics()) = %d, want %d", got, want)
	}
}

func TestTopicByCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code string
		want string
	}{
		{"basics", "簿記の基本原理"},
		{"cash", "現金預金"},
		{"corp_equity", "株式会社会計"},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			t.Parallel()
			tp, ok := domain.TopicByCode(c.code)
			if !ok {
				t.Fatalf("TopicByCode(%q) ok=false", c.code)
			}
			if tp.Name != c.want {
				t.Fatalf("TopicByCode(%q).Name = %q, want %q", c.code, tp.Name, c.want)
			}
		})
	}
}

func TestTopicByCodeUnknown(t *testing.T) {
	t.Parallel()
	if _, ok := domain.TopicByCode("nonexistent"); ok {
		t.Fatalf("TopicByCode(\"nonexistent\") ok=true, want false")
	}
}
```

- [ ] **Step 2: 失敗確認**

```bash
go test ./internal/domain/...
```

Expected: ビルドエラー (Topics, TopicByCode 未定義)。

- [ ] **Step 3: 実装**

`internal/domain/topic.go`:
```go
package domain

// Topic は日商簿記 3 級の論点を 1 つ表す。
type Topic struct {
	ID   int64
	Code string
	Name string
	Ord  int
}

// topics は固定の 15 論点。出題区分表 2022 年度版 (2026 年度試験適用) に準拠する。
var topics = []Topic{
	{ID: 1, Code: "basics", Name: "簿記の基本原理", Ord: 1},
	{ID: 2, Code: "books_slips", Name: "帳簿・伝票・証ひょう", Ord: 2},
	{ID: 3, Code: "cash", Name: "現金預金", Ord: 3},
	{ID: 4, Code: "ar_ap", Name: "売掛金と買掛金", Ord: 4},
	{ID: 5, Code: "other_claims", Name: "その他の債権と債務", Ord: 5},
	{ID: 6, Code: "bills", Name: "手形・電子記録債権/債務", Ord: 6},
	{ID: 7, Code: "credit_ar", Name: "クレジット売掛金", Ord: 7},
	{ID: 8, Code: "allowance", Name: "貸倒引当金 (実績法)", Ord: 8},
	{ID: 9, Code: "merchandise", Name: "商品売買 (3 分法)", Ord: 9},
	{ID: 10, Code: "fixed_assets", Name: "有形固定資産", Ord: 10},
	{ID: 11, Code: "income_expense", Name: "収益と費用", Ord: 11},
	{ID: 12, Code: "taxes", Name: "税金", Ord: 12},
	{ID: 13, Code: "closing", Name: "決算整理", Ord: 13},
	{ID: 14, Code: "statements", Name: "試算表/精算表/財務諸表", Ord: 14},
	{ID: 15, Code: "corp_equity", Name: "株式会社会計", Ord: 15},
}

// Topics は 15 論点のコピーを順序付きで返す。
func Topics() []Topic {
	out := make([]Topic, len(topics))
	copy(out, topics)
	return out
}

// TopicByCode は code に対応する論点を返す。見つからないとき ok=false。
func TopicByCode(code string) (Topic, bool) {
	for _, t := range topics {
		if t.Code == code {
			return t, true
		}
	}
	return Topic{}, false
}
```

- [ ] **Step 4: テスト Pass 確認**

```bash
go test -count=1 -race ./internal/domain/...
```

Expected: 3 テスト PASS。

- [ ] **Step 5: コミット**

```bash
git add internal/domain/topic.go internal/domain/topic_test.go
git -c commit.gpgsign=false commit -m "feat(domain): add 15 fixed topics with lookup"
```

---

### Task 4: Question と AnswerPayload

**Files:**
- Create: `internal/domain/question.go`
- Test: `internal/domain/question_test.go`

- [ ] **Step 1: テストを書く**

`internal/domain/question_test.go`:
```go
package domain_test

import (
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

func TestQuestionTypeValid(t *testing.T) {
	t.Parallel()
	valid := []domain.QuestionType{
		domain.QuestionTypeJournal,
		domain.QuestionTypeLedger,
		domain.QuestionTypeSubbook,
		domain.QuestionTypeTrialBalance,
		domain.QuestionTypeWorksheet,
		domain.QuestionTypeFS,
		domain.QuestionTypeSlip,
	}
	for _, qt := range valid {
		t.Run(string(qt), func(t *testing.T) {
			t.Parallel()
			if !qt.IsValid() {
				t.Fatalf("%q.IsValid() = false", qt)
			}
		})
	}
	if domain.QuestionType("bogus").IsValid() {
		t.Fatalf("bogus.IsValid() = true")
	}
}

func TestQuizModeValid(t *testing.T) {
	t.Parallel()
	for _, m := range []domain.QuizMode{
		domain.QuizModeSRS, domain.QuizModeSequential, domain.QuizModeRandom,
	} {
		if !m.IsValid() {
			t.Fatalf("%q.IsValid() = false", m)
		}
	}
	if domain.QuizMode("nope").IsValid() {
		t.Fatalf("nope.IsValid() = true")
	}
}
```

- [ ] **Step 2: 失敗確認**

```bash
go test ./internal/domain/...
```

Expected: ビルドエラー。

- [ ] **Step 3: 実装**

`internal/domain/question.go`:
```go
package domain

import "time"

// QuestionType は出題形式を表す。Spec 5.2 の question_type 列に対応。
type QuestionType string

const (
	QuestionTypeJournal      QuestionType = "journal"       // 仕訳
	QuestionTypeLedger       QuestionType = "ledger"        // 勘定記入
	QuestionTypeSubbook      QuestionType = "subbook"       // 補助簿
	QuestionTypeTrialBalance QuestionType = "trial_balance" // 試算表
	QuestionTypeWorksheet    QuestionType = "worksheet"     // 精算表
	QuestionTypeFS           QuestionType = "fs"            // 財務諸表
	QuestionTypeSlip         QuestionType = "slip"          // 伝票
)

// IsValid は既知の QuestionType か判定する。
func (q QuestionType) IsValid() bool {
	switch q {
	case QuestionTypeJournal, QuestionTypeLedger, QuestionTypeSubbook,
		QuestionTypeTrialBalance, QuestionTypeWorksheet, QuestionTypeFS, QuestionTypeSlip:
		return true
	}
	return false
}

// QuizMode は次問題抽選戦略を表す。
type QuizMode string

const (
	QuizModeSRS        QuizMode = "srs"
	QuizModeSequential QuizMode = "sequential"
	QuizModeRandom     QuizMode = "random"
)

// IsValid は既知の QuizMode か判定する。
func (m QuizMode) IsValid() bool {
	switch m {
	case QuizModeSRS, QuizModeSequential, QuizModeRandom:
		return true
	}
	return false
}

// JournalEntry は仕訳 1 行を表す (借方 or 貸方)。
type JournalEntry struct {
	Account string // 勘定科目
	Amount  int64  // 円 (整数のみ)
}

// AnswerPayload は採点対象の解答ペイロード。問題形式により Journal / Choice / Text を使い分ける。
type AnswerPayload struct {
	Type    QuestionType
	Debits  []JournalEntry // journal 用
	Credits []JournalEntry // journal 用
	Choice  string         // multiple choice や穴埋め用
	Text    string         // 自由記述用 (未使用、将来拡張)
}

// Question は問題マスタの 1 件。payload_json / answer_json は JSON 文字列のまま保持し、
// repo 層で AnswerPayload へデコードする。
type Question struct {
	ID            int64
	Code          string
	TopicID       int64
	QuestionType  QuestionType
	Difficulty    int
	Prompt        string
	PayloadJSON   string
	AnswerJSON    string
	Explanation   string
	ReferencesJSON string
	CreatedAt     time.Time
}

// QuestionFilter は QuestionRepository.Search の引数。
type QuestionFilter struct {
	TopicCodes []string
	Types      []QuestionType
	SetCode    string
	Limit      int
	Offset     int
}
```

- [ ] **Step 4: テスト Pass 確認とコミット**

```bash
go test -count=1 -race ./internal/domain/...
git add internal/domain/question.go internal/domain/question_test.go
git -c commit.gpgsign=false commit -m "feat(domain): add Question, AnswerPayload, QuizMode"
```

---

### Task 5: Attempt と統計 DTO

**Files:**
- Create: `internal/domain/attempt.go`

- [ ] **Step 1: 実装 (テストは Phase 3 の repo 側で連動カバー)**

`internal/domain/attempt.go`:
```go
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
type GradedAttempt struct {
	Attempt     Attempt
	IsCorrect   bool
	Explanation string
	NextDueAt   time.Time
}
```

- [ ] **Step 2: ビルドとコミット**

```bash
go build ./internal/domain/...
git add internal/domain/attempt.go
git -c commit.gpgsign=false commit -m "feat(domain): add Attempt and stats DTOs"
```

---

### Task 6: 問題セット

**Files:**
- Create: `internal/domain/set.go`

- [ ] **Step 1: 実装**

`internal/domain/set.go`:
```go
package domain

// QuestionSet は学習モードに対応する問題集合。
type QuestionSet struct {
	ID          int64
	Code        string
	Name        string
	Description string
	TargetSize  int
}

// 既定 3 セットの code。
const (
	SetCodeCore          = "core_300"
	SetCodeJournal       = "journal_240"
	SetCodeComprehensive = "comprehensive_300"
)

// DefaultSetCodes は UI で選択肢に並べる順。
func DefaultSetCodes() []string {
	return []string{SetCodeCore, SetCodeJournal, SetCodeComprehensive}
}
```

- [ ] **Step 2: コミット**

```bash
git add internal/domain/set.go
git -c commit.gpgsign=false commit -m "feat(domain): add QuestionSet with default codes"
```

---

### Task 7: SRS Grade と Map (TDD)

**Files:**
- Create: `internal/domain/srs/grade.go`
- Test: `internal/domain/srs/grade_test.go`

- [ ] **Step 1: テストを書く**

`internal/domain/srs/grade_test.go`:
```go
package srs_test

import (
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/srs"
)

func TestGradeFromIncorrect(t *testing.T) {
	t.Parallel()
	if got := srs.GradeFromResult(false, 1000); got != srs.GradeWorst {
		t.Fatalf("incorrect grade = %d, want %d", got, srs.GradeWorst)
	}
}

func TestGradeFromCorrectFast(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ms   int
		want srs.Grade
	}{
		{1000, srs.GradePerfect}, // < 5s
		{4999, srs.GradePerfect},
		{5000, srs.GradeGood},  // 5..15
		{14999, srs.GradeGood},
		{15000, srs.GradeSlow}, // >= 15
		{99999, srs.GradeSlow},
	}
	for _, c := range cases {
		got := srs.GradeFromResult(true, c.ms)
		if got != c.want {
			t.Fatalf("GradeFromResult(true, %d) = %d, want %d", c.ms, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 失敗確認**

```bash
go test ./internal/domain/srs/...
```

Expected: ビルドエラー。

- [ ] **Step 3: 実装**

`internal/domain/srs/grade.go`:
```go
// Package srs は SM-2 風の間隔反復アルゴリズムをコレクションする。
package srs

// Grade は SM-2 の品質スコア (0..5)。
type Grade int

const (
	GradeWorst   Grade = 0
	GradeFail    Grade = 2
	GradeSlow    Grade = 3
	GradeGood    Grade = 4
	GradePerfect Grade = 5
)

// GradeFromResult は正誤と解答時間 (ms) から Grade を導く。
//   - 不正解 -> 0
//   - 正解 5s 未満 -> 5
//   - 正解 5s 以上 15s 未満 -> 4
//   - 正解 15s 以上 -> 3
func GradeFromResult(correct bool, durationMs int) Grade {
	if !correct {
		return GradeWorst
	}
	switch {
	case durationMs < 5000:
		return GradePerfect
	case durationMs < 15000:
		return GradeGood
	default:
		return GradeSlow
	}
}
```

- [ ] **Step 4: テスト Pass 確認とコミット**

```bash
go test -count=1 -race ./internal/domain/srs/...
git add internal/domain/srs/
git -c commit.gpgsign=false commit -m "feat(domain/srs): add Grade and GradeFromResult"
```

---

### Task 8: SRS State と SM-2 Next 関数 (TDD)

**Files:**
- Create: `internal/domain/srs/state.go`
- Create: `internal/domain/srs/sm2.go`
- Test: `internal/domain/srs/sm2_test.go`

- [ ] **Step 1: テストを書く**

`internal/domain/srs/sm2_test.go`:
```go
package srs_test

import (
	"math"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/srs"
)

func TestInitialState(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)
	s := srs.NewState(now)
	if s.EFactor != 2.5 {
		t.Fatalf("EFactor = %f, want 2.5", s.EFactor)
	}
	if s.Repetitions != 0 {
		t.Fatalf("Repetitions = %d, want 0", s.Repetitions)
	}
	if !s.DueAt.Equal(now) {
		t.Fatalf("DueAt = %v, want %v", s.DueAt, now)
	}
}

func TestNextOnPerfect(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)
	s := srs.NewState(now)

	s1 := srs.Next(s, srs.GradePerfect, now)
	if s1.IntervalDays != 1 {
		t.Fatalf("first interval = %d, want 1", s1.IntervalDays)
	}
	if s1.Repetitions != 1 {
		t.Fatalf("repetitions after first = %d, want 1", s1.Repetitions)
	}
	if !s1.DueAt.Equal(now.AddDate(0, 0, 1)) {
		t.Fatalf("due = %v, want %v", s1.DueAt, now.AddDate(0, 0, 1))
	}

	s2 := srs.Next(s1, srs.GradePerfect, now.AddDate(0, 0, 1))
	if s2.IntervalDays != 6 {
		t.Fatalf("second interval = %d, want 6", s2.IntervalDays)
	}

	s3 := srs.Next(s2, srs.GradePerfect, now.AddDate(0, 0, 7))
	want3 := int(math.Round(6 * s2.EFactor))
	if s3.IntervalDays != want3 {
		t.Fatalf("third interval = %d, want %d", s3.IntervalDays, want3)
	}
}

func TestNextOnFailResetsRepetition(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)
	s := srs.NewState(now)
	s = srs.Next(s, srs.GradePerfect, now)
	s = srs.Next(s, srs.GradePerfect, now.AddDate(0, 0, 1))
	if s.Repetitions != 2 {
		t.Fatalf("setup: Repetitions = %d, want 2", s.Repetitions)
	}

	s = srs.Next(s, srs.GradeWorst, now.AddDate(0, 0, 7))
	if s.Repetitions != 0 {
		t.Fatalf("Repetitions after fail = %d, want 0", s.Repetitions)
	}
	if s.IntervalDays != 1 {
		t.Fatalf("IntervalDays after fail = %d, want 1", s.IntervalDays)
	}
}

func TestEFactorClampedAtMin(t *testing.T) {
	t.Parallel()
	now := time.Now()
	s := srs.NewState(now)
	for i := range 30 {
		s = srs.Next(s, srs.GradeWorst, now.AddDate(0, 0, i))
	}
	if s.EFactor < 1.3 {
		t.Fatalf("EFactor = %f, want >= 1.3", s.EFactor)
	}
}
```

- [ ] **Step 2: 失敗確認**

```bash
go test ./internal/domain/srs/...
```

Expected: ビルドエラー。

- [ ] **Step 3: state.go の実装**

`internal/domain/srs/state.go`:
```go
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
```

- [ ] **Step 4: sm2.go の実装**

`internal/domain/srs/sm2.go`:
```go
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
```

- [ ] **Step 5: テスト Pass 確認とコミット**

```bash
go test -count=1 -race ./internal/domain/srs/...
git add internal/domain/srs/state.go internal/domain/srs/sm2.go internal/domain/srs/sm2_test.go
git -c commit.gpgsign=false commit -m "feat(domain/srs): add SM-2 Next with E-factor clamp"
```

---

### Task 9: 仕訳採点関数 (TDD)

**Files:**
- Create: `internal/domain/grading/grading.go`
- Test: `internal/domain/grading/grading_test.go`

- [ ] **Step 1: テストを書く**

`internal/domain/grading/grading_test.go`:
```go
package grading_test

import (
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/grading"
)

func TestJournalCorrectAnyOrder(t *testing.T) {
	t.Parallel()
	want := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "現金", Amount: 10000}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 10000}},
	}
	got := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "現金", Amount: 10000}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 10000}},
	}
	if !grading.IsCorrect(want, got) {
		t.Fatalf("expected correct match")
	}
}

func TestJournalReorderedStillCorrect(t *testing.T) {
	t.Parallel()
	want := domain.AnswerPayload{
		Type: domain.QuestionTypeJournal,
		Debits: []domain.JournalEntry{
			{Account: "現金", Amount: 7000},
			{Account: "売掛金", Amount: 3000},
		},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 10000}},
	}
	got := domain.AnswerPayload{
		Type: domain.QuestionTypeJournal,
		Debits: []domain.JournalEntry{
			{Account: "売掛金", Amount: 3000},
			{Account: "現金", Amount: 7000},
		},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 10000}},
	}
	if !grading.IsCorrect(want, got) {
		t.Fatalf("expected correct (order shuffled)")
	}
}

func TestJournalEmptyRowsIgnored(t *testing.T) {
	t.Parallel()
	want := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "現金", Amount: 5000}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 5000}},
	}
	got := domain.AnswerPayload{
		Type: domain.QuestionTypeJournal,
		Debits: []domain.JournalEntry{
			{Account: "現金", Amount: 5000},
			{Account: "", Amount: 0},
		},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 5000}},
	}
	if !grading.IsCorrect(want, got) {
		t.Fatalf("expected correct (empty rows ignored)")
	}
}

func TestJournalAmountMismatchIncorrect(t *testing.T) {
	t.Parallel()
	want := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "現金", Amount: 5000}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 5000}},
	}
	got := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "現金", Amount: 4999}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 4999}},
	}
	if grading.IsCorrect(want, got) {
		t.Fatalf("expected incorrect (amount mismatch)")
	}
}

func TestJournalAccountMismatchIncorrect(t *testing.T) {
	t.Parallel()
	want := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "現金", Amount: 5000}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 5000}},
	}
	got := domain.AnswerPayload{
		Type:    domain.QuestionTypeJournal,
		Debits:  []domain.JournalEntry{{Account: "当座預金", Amount: 5000}},
		Credits: []domain.JournalEntry{{Account: "売上", Amount: 5000}},
	}
	if grading.IsCorrect(want, got) {
		t.Fatalf("expected incorrect (account mismatch)")
	}
}

func TestChoiceMatch(t *testing.T) {
	t.Parallel()
	want := domain.AnswerPayload{Type: domain.QuestionTypeSubbook, Choice: "A"}
	got := domain.AnswerPayload{Type: domain.QuestionTypeSubbook, Choice: "a"}
	if !grading.IsCorrect(want, got) {
		t.Fatalf("expected case-insensitive choice match")
	}
}
```

- [ ] **Step 2: 失敗確認**

```bash
go test ./internal/domain/grading/...
```

Expected: ビルドエラー。

- [ ] **Step 3: 実装**

`internal/domain/grading/grading.go`:
```go
// Package grading は提出解答の正誤判定を提供する純関数集。
package grading

import (
	"sort"
	"strings"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// IsCorrect は want (正答) と got (提出) が一致するか判定する。
// 仕訳は (科目, 金額) の多重集合一致、選択肢は大文字小文字を無視した一致、テキストはトリム後の一致。
func IsCorrect(want, got domain.AnswerPayload) bool {
	if want.Type != got.Type {
		return false
	}
	switch want.Type {
	case domain.QuestionTypeJournal, domain.QuestionTypeLedger, domain.QuestionTypeSlip:
		return entrySetEqual(want.Debits, got.Debits) && entrySetEqual(want.Credits, got.Credits)
	case domain.QuestionTypeSubbook, domain.QuestionTypeTrialBalance,
		domain.QuestionTypeWorksheet, domain.QuestionTypeFS:
		if want.Choice != "" || got.Choice != "" {
			return strings.EqualFold(strings.TrimSpace(want.Choice), strings.TrimSpace(got.Choice))
		}
		return strings.TrimSpace(want.Text) == strings.TrimSpace(got.Text)
	default:
		return false
	}
}

func entrySetEqual(a, b []domain.JournalEntry) bool {
	na := normalize(a)
	nb := normalize(b)
	if len(na) != len(nb) {
		return false
	}
	for i := range na {
		if na[i] != nb[i] {
			return false
		}
	}
	return true
}

// normalize は空行を除き、(科目, 金額) を辞書順にソートしたコピーを返す。
func normalize(entries []domain.JournalEntry) []domain.JournalEntry {
	out := make([]domain.JournalEntry, 0, len(entries))
	for _, e := range entries {
		acct := strings.TrimSpace(e.Account)
		if acct == "" && e.Amount == 0 {
			continue
		}
		out = append(out, domain.JournalEntry{Account: acct, Amount: e.Amount})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Account != out[j].Account {
			return out[i].Account < out[j].Account
		}
		return out[i].Amount < out[j].Amount
	})
	return out
}
```

- [ ] **Step 4: テスト Pass 確認とコミット**

```bash
go test -count=1 -race ./internal/domain/grading/...
git add internal/domain/grading/
git -c commit.gpgsign=false commit -m "feat(domain/grading): add journal/choice grading"
```

---

### Task 10: JWT クレーム

**Files:**
- Create: `internal/domain/jwt.go`

- [ ] **Step 1: 実装**

`internal/domain/jwt.go`:
```go
package domain

import "time"

// JWTClaims は HS256 JWT の本アプリ向けクレーム。
// 標準クレーム名は RFC 7519 (sub/iat/exp/iss/aud/jti) に従う。
type JWTClaims struct {
	Subject   int64     // sub: user_id
	Issuer    string    // iss: "boki3-quiz"
	Audience  string    // aud: "api"
	IssuedAt  time.Time // iat
	ExpiresAt time.Time // exp
	JTI       string    // jti
}
```

- [ ] **Step 2: ビルドとコミット**

```bash
go build ./internal/domain/...
git add internal/domain/jwt.go
git -c commit.gpgsign=false commit -m "feat(domain): add JWT claims value object"
```

---

### Task 11: Phase 2 全体動作確認

- [ ] **Step 1: テスト + 品質ゲート**

```bash
go test -count=1 -race -cover ./internal/domain/...
bash scripts/quality-gate.sh
```

Expected: 全テスト PASS、`all quality checks passed`。

- [ ] **Step 2: コミット (もし未追跡があれば)**

```bash
git status
```

Expected: clean。

## Phase 2 完了条件

1. `bash scripts/quality-gate.sh` が exit 0
2. `internal/domain/**` のテストカバレッジが目安 80% 以上 (純関数のため自然と高い)
3. すべて git にコミット済み

完了後、Phase 3 (SQLite Repository) の計画書作成へ進む。
