# boki3-quiz-go-htmx 設計書

- 作成日: 2026-05-24
- 対象: 日商簿記検定 3 級 学習用 Web アプリ
- 配置: `/Users/yujiokamoto/devs/golang/boki3-quiz-go-htmx`

## 1. 概要

日商簿記検定 3 級の問題集 Web アプリです。約 300 問のコアセット (バランス型) に加え、仕訳特化セット 240 問と全範囲網羅セット 300 問を選択して学習でき、ユニーク問題総数は約 430 問です。回答後は解説ページを表示し、履歴の個別/一括削除、論点別正解率と日次正解率の SVG グラフ、SM-2 風 SRS を備えます。

UI は和モダン (紙×墨×朱) で、Go の標準ライブラリと HTMX + Alpine.js のみで実装します。

## 2. スコープと非スコープ

### 2.1 スコープ

- 日商簿記 3 級の出題範囲 (2022 年度版区分表、2026 年度試験適用) の論点を網羅した問題集
- ユーザー認証 (登録/ログイン/パスワード変更/ログアウト)
- 学習モード 3 種 (実試験準拠/仕訳特化/全範囲網羅) の選択
- 弱点重視 + SM-2 風 SRS による次問題選定
- 解説ページと履歴管理 (個別/一括削除)
- 論点別正解率と日次正解率の SVG グラフ
- Web UI (Cookie セッション) と JSON API (JWT Bearer) の二重提供
- CORS、レートリミット、CSRF、OWASP Top 10 対策
- pre-commit + CI 同形の品質ゲート全 Pass
- フロントエンドのユニットテスト + E2E テスト
- カバレッジ目標 80% (厳密でない目安)

### 2.2 非スコープ

- 2 級以上の論点
- マルチテナント機能
- メール送信 (パスワードリセット等)
- 課金/サブスクリプション
- モバイルアプリ (PWA は対象外、レスポンシブ対応のみ)
- OIDC/OAuth プロバイダ統合

## 3. 確定要件

| 項目 | 確定値 |
|---|---|
| アプリ名 | `boki3-quiz-go-htmx` |
| 言語/ランタイム | Go 1.25 |
| DB | SQLite (WAL モード、`?` プレースホルダのみ) |
| フロントエンド | HTMX + Alpine.js (セルフホスト) |
| 認証 | ユーザー名+パスワード+scrypt+セッション Cookie |
| API 認証 | JWT Bearer (HS256、自作) |
| 認証範囲 | アカウント認証付き |
| 問題セット | コア 300 + 仕訳 240 + 網羅 300 (重複含む、ユニーク約 430) |
| 学習モード | SM-2 風 SRS + 弱点重視 |
| UI テーマ | 和モダン (紙×墨×朱) |
| 削除 | 学習履歴の個別/一括削除 |
| グラフ | サーバ側 SVG 生成 + HTMX 返却 |
| パスワード変更 | 旧パスワード検証 + 他セッション全破棄 |
| レートリミット | グローバルインメモリ + エンドポイント別多階層 |

## 4. 全体アーキテクチャ

### 4.1 レイヤ構成

すべての層間は `internal/port` に定義した interface を介して接続します。具体型は受け取りません。

```
cmd/boki3-quiz/                  main, シグナル制御、DI 配線
internal/
  domain/                        ドメイン型と純粋ロジック (依存ゼロ)
  port/                          interface 定義のみ
  repo/sqlite/                   SQLite による Repository 実装
  service/                       ユースケース (Service) 実装
  transport/http/                handler, middleware, router, view
    handler/                     auth, quiz, srs, stats, account, admin, api_v1
    middleware/                  chain, recover, requestid, ratelimit, csrf,
                                 session, jwt, cors, bodylimit, security_headers
    view/                        html/template と SVG ジェネレータ
  pkg/                           httpx, logger, idgen, crypto, errors, htmx
migrations/                      sqlite 用 .sql スキーマ
internal/data/seed/              問題シード JSON (embed.FS)
web/                             static (css, alpine.js, fonts), templates
tests/
  unit, integration, frontend, e2e
scripts/
  quality-gate.sh, hooks/, verify-hardening.sh
```

### 4.2 外部依存方針

Go 標準ライブラリのみを基本とし、以下の例外のみを許容します。

| 依存 | 用途 | 理由 |
|---|---|---|
| `modernc.org/sqlite` | `database/sql` 用 SQLite ドライバ | 標準に SQLite ドライバが無く、Pure Go で CGO 不要 |
| `golang.org/x/crypto/scrypt` | パスワード派生 | 標準に scrypt が無く、暗号は自作禁止 (OWASP) |

JWT (HS256) は `crypto/hmac` + `encoding/base64` + `encoding/json` で自作します。

## 5. データモデル

すべての SQL は `?` プレースホルダのみで実装し、`fmt.Sprintf` での値埋め込みは禁止です。トランザクションは `BEGIN IMMEDIATE`、PRAGMA は `journal_mode=WAL`、`foreign_keys=ON`、`busy_timeout=5000` を起動時に設定します。

### 5.1 ER 概要

```
users 1---* sessions
users 1---* jwt_revocations
users 1---* attempts *---1 questions
users 1---* srs_states *---1 questions
questions *---* question_sets (via question_set_members)
questions *---1 topics
```

### 5.2 スキーマ定義

```sql
-- users
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

-- sessions (Web UI)
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

-- JWT 失効テーブル
CREATE TABLE jwt_revocations (
  jti TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL,
  revoked_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX idx_jwt_rev_user ON jwt_revocations(user_id);

-- 15 大論点
CREATE TABLE topics (
  id INTEGER PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  ord INTEGER NOT NULL
);

-- 問題マスタ
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

-- 問題セット
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

-- 回答履歴
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

-- SRS 状態
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

## 6. 問題セットと 15 論点

### 6.1 問題セット構成

| code | 用途 | 内訳 |
|---|---|---|
| `core_300` | バランス型 | 仕訳 150 + 補助簿/勘定記入/伝票 75 + 試算表/精算表/財務諸表 75 |
| `journal_240` | 仕訳特化 | コア仕訳 150 + 追加仕訳 90 |
| `comprehensive_300` | 全範囲網羅 | 15 論点 × 20 問 |

合計 840 問のメンバシップを 3 セットで共有し、ユニーク問題は約 430 問を想定します (シード作成段階で論点別の最終件数を確定)。

各論点内で「数字違いの類似問題」を 3〜5 問単位でクラスタリングし、定着を促します。

### 6.2 15 論点 (区分表 2022 年度版・2026 年度試験適用)

| code | 名称 | 公式区分参照 |
|---|---|---|
| `basics` | 簿記の基本原理 | 第一 1〜3 |
| `books_slips` | 帳簿・伝票・証ひょう | 第一 4〜5 |
| `cash` | 現金預金 | 第二 1 |
| `ar_ap` | 売掛金と買掛金 | 第二 3 |
| `other_claims` | その他の債権と債務 | 第二 4 |
| `bills` | 手形・電子記録債権/債務 | 第二 5 |
| `credit_ar` | クレジット売掛金 | 第二 6 |
| `allowance` | 貸倒引当金 (実績法) | 第二 7 |
| `merchandise` | 商品売買 (3 分法) | 第二 9 |
| `fixed_assets` | 有形固定資産 | 第二 12 |
| `income_expense` | 収益と費用 | 第二 20 |
| `taxes` | 税金 | 第二 21 |
| `closing` | 決算整理 | 第三 1〜6, 8 |
| `statements` | 試算表/精算表/財務諸表 | 第三 2, 9 |
| `corp_equity` | 株式会社会計 (資本金/繰越利益剰余金) | 第四 1, 3, 4 |

## 7. interface 定義

### 7.1 Repository 層

```go
// internal/port/repo.go
type UserRepository interface {
    Create(ctx context.Context, u *domain.User) error
    FindByUsername(ctx context.Context, username string) (*domain.User, error)
    FindByID(ctx context.Context, id int64) (*domain.User, error)
    UpdatePassword(ctx context.Context, id int64, hash, salt []byte, params string, at time.Time) error
}

type SessionRepository interface {
    Create(ctx context.Context, s *domain.Session) error
    FindByID(ctx context.Context, id string) (*domain.Session, error)
    Touch(ctx context.Context, id string, lastSeen time.Time) error
    Delete(ctx context.Context, id string) error
    DeleteAllForUser(ctx context.Context, userID int64) error
    DeleteAllForUserExcept(ctx context.Context, userID int64, keepID string) error
    PurgeExpired(ctx context.Context, now time.Time) (int, error)
}

type JWTRevocationRepository interface {
    Revoke(ctx context.Context, jti string, userID int64, expiresAt time.Time) error
    IsRevoked(ctx context.Context, jti string) (bool, error)
    RevokeAllForUser(ctx context.Context, userID int64, now time.Time) error
    PurgeExpired(ctx context.Context, now time.Time) (int, error)
}

type QuestionRepository interface {
    GetByID(ctx context.Context, id int64) (*domain.Question, error)
    ListBySet(ctx context.Context, setCode string) ([]domain.Question, error)
    Search(ctx context.Context, filter domain.QuestionFilter) ([]domain.Question, error)
}

type AttemptRepository interface {
    Create(ctx context.Context, a *domain.Attempt) error
    ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.Attempt, error)
    DeleteByID(ctx context.Context, userID, attemptID int64) error
    DeleteAllForUser(ctx context.Context, userID int64) error
    StatsByTopic(ctx context.Context, userID int64) ([]domain.TopicStat, error)
    DailyAccuracy(ctx context.Context, userID int64, days int) ([]domain.DailyAccuracy, error)
}

type SRSStateRepository interface {
    Upsert(ctx context.Context, s *domain.SRSState) error
    DueForUser(ctx context.Context, userID int64, now time.Time, limit int) ([]domain.SRSState, error)
    Get(ctx context.Context, userID, questionID int64) (*domain.SRSState, error)
    DeleteAllForUser(ctx context.Context, userID int64) error
}
```

### 7.2 Service 層

```go
// internal/port/service.go
type AuthService interface {
    Register(ctx context.Context, username, password string) (*domain.User, error)
    Login(ctx context.Context, username, password, ip, userAgent string) (*domain.Session, error)
    Logout(ctx context.Context, sessionID string) error
    ChangePassword(ctx context.Context, userID int64, currentPW, newPW string, currentSessionID string) error
    AuthenticateSession(ctx context.Context, sessionID string) (*domain.User, *domain.Session, error)
}

type APITokenService interface {
    Issue(ctx context.Context, userID int64) (token string, expiresAt time.Time, err error)
    Verify(ctx context.Context, token string) (*domain.JWTClaims, error)
    RevokeJTI(ctx context.Context, jti string, userID int64, expiresAt time.Time) error
    RevokeAllForUser(ctx context.Context, userID int64) error
}

type QuizService interface {
    NextQuestion(ctx context.Context, userID int64, setCode string, mode domain.QuizMode) (*domain.Question, error)
    Submit(ctx context.Context, userID, questionID int64, setCode string, submitted domain.AnswerPayload, durationMs int) (*domain.GradedAttempt, error)
}

type StatsService interface {
    Summary(ctx context.Context, userID int64) (*domain.StatsSummary, error)
    TopicBreakdown(ctx context.Context, userID int64) ([]domain.TopicStat, error)
    DailyAccuracySVG(ctx context.Context, userID int64, days int) (string, error)
}
```

### 7.3 ユーティリティ

```go
// internal/port/util.go
type Clock interface { Now() time.Time }
type IDGenerator interface {
    NewToken(bytes int) (string, error)
    NewUUID() string
}
type PasswordHasher interface {
    Hash(plain string) (hash, salt []byte, params string, err error)
    Verify(plain string, hash, salt []byte, params string) (bool, error)
}
type JWTSigner interface {
    Sign(claims domain.JWTClaims) (string, error)
    Parse(token string) (domain.JWTClaims, error)
}
type RateLimiter interface { Allow(key string) bool }
```

## 8. HTTP エンドポイント

### 8.1 Web UI (HTMX、Cookie セッション + CSRF)

| メソッド | パス | 説明 |
|---|---|---|
| GET | `/` | 未ログイン→`/login`、ログイン済→`/home` |
| GET | `/login` | ログイン画面 |
| POST | `/login` | ログイン処理 |
| GET | `/register` | 登録画面 |
| POST | `/register` | 登録処理 |
| POST | `/logout` | ログアウト |
| GET | `/home` | ダッシュボード |
| GET | `/quiz/start?set=core_300&mode=srs` | セッション開始 |
| GET | `/quiz/next` | 次の問題 (HTMX) |
| POST | `/quiz/submit` | 回答送信 (HTMX で結果と解説を返却) |
| GET | `/explain/{question_id}` | 解説単体 |
| GET | `/history` | 履歴一覧 |
| DELETE | `/history/{attempt_id}` | 個別履歴削除 |
| POST | `/history/clear` | 履歴一括削除 |
| GET | `/stats` | 統計画面 |
| GET | `/account` | 設定画面 |
| POST | `/account/password` | パスワード変更 |
| POST | `/account/api-token` | JWT 発行 (1 回限り表示) |
| POST | `/account/api-token/revoke-all` | API トークン全失効 |

### 8.2 API v1 (JWT Bearer)

| メソッド | パス | 説明 |
|---|---|---|
| POST | `/api/v1/auth/token` | username+password で 24h JWT 発行 |
| GET | `/api/v1/me` | 自分の情報 |
| GET | `/api/v1/quiz/next?set=core_300&mode=srs` | 次問題 (JSON) |
| POST | `/api/v1/quiz/submit` | 回答送信 (JSON) |
| GET | `/api/v1/stats/summary` | 統計サマリ (JSON) |
| GET | `/api/v1/stats/daily?days=30` | 日次正解率 (JSON) |
| DELETE | `/api/v1/history/{attempt_id}` | 個別削除 |
| POST | `/api/v1/history/clear` | 一括削除 |

API レスポンスは以下の共通エンベロープです。

```json
{"success": true, "data": {...}, "error": null, "request_id": "..."}
```

### 8.3 ヘルス/メタ

| メソッド | パス | 説明 |
|---|---|---|
| GET | `/healthz` | liveness (DB ping を含む) |
| GET | `/readyz` | readiness |
| GET | `/version` | ビルド情報 |

## 9. セキュリティ

### 9.1 ミドルウェアチェーン

外側から内側へ以下の順で適用します。

```
recover → request_id → access_log → body_limit(1MB) →
security_headers → cors → ratelimit(global) →
[router]
  /login         → ratelimit(per_ip_user 5/10min) → csrf
  /api/v1/auth/* → ratelimit(per_ip 10/10min)
  /api/v1/*      → jwt_auth → ratelimit(per_user token bucket 60/min)
  /              → session_auth → csrf
```

### 9.2 Security Headers (全レスポンス)

```
Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; object-src 'none'
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: camera=(), microphone=(), geolocation=()
X-Frame-Options: DENY
Cache-Control: no-store (認証画面と API)
```

Alpine.js はセルフホストで CDN を使わないため、`script-src 'self'` で成立します。

### 9.3 OWASP Top 10 (2021) 対策マップ

| ID | 項目 | 対策 |
|---|---|---|
| A01 Broken Access Control | 全 handler で認証必須化、削除は user_id 一致を WHERE で要求 |
| A02 Cryptographic Failures | scrypt + 256bit ランダム salt、JWT は HS256 (32B 以上シークレット、env 必須)、CSRF/session ID は 256bit CSPRNG |
| A03 Injection | `?` プレースホルダのみ、`check_no_sql_sprintf.sh` で grep 確認、staticcheck SA1000 有効 |
| A04 Insecure Design | レイヤ分離、SRS は domain 純関数、副作用は service に隔離 |
| A05 Security Misconfiguration | Security Headers、CORS allowlist、Cache-Control: no-store、SQLite の WAL+FK+busy_timeout |
| A06 Vulnerable & Outdated | govulncheck を quality-gate.sh に組込 |
| A07 Identification & Authentication | scrypt、5/10min ロックアウト、旧パスワード必須、他セッション全破棄、JWT jti 失効 |
| A08 Software & Data Integrity | embed.FS で問題シードを内包、SHA256 を起動時に検証 (integrity.json) |
| A09 Logging & Monitoring | request_id、失敗ログイン記録、パスワード変更/トークン失効の監査ログ |
| A10 SSRF | 外部 fetch 機能を持たない設計 |

### 9.4 レートリミット仕様

| 対象 | アルゴリズム | しきい値 |
|---|---|---|
| グローバル (IP) | sliding window | 120 req / min / IP |
| `/login`、`/register` | 固定ウィンドウ | 5 試行 / 10 min / (IP+username) |
| `/api/v1/auth/token` | 固定ウィンドウ | 10 試行 / 10 min / IP |
| `/api/v1/*` (認証後) | token bucket | 60 req / min / user_id |
| Body 上限 | http.MaxBytesReader | 1 MB |

実装は `sync.Map` + `sync.Mutex` の in-memory です。プロセス再起動でリセットされます。

### 9.5 CORS

- 既定は許可 Origin 空 (同一オリジンのみ)
- `CORS_ALLOWED_ORIGINS` 環境変数で allowlist を設定
- `Access-Control-Allow-Credentials` は付与しない (API は JWT で stateless)
- Preflight OPTIONS は middleware で 204 を返す

### 9.6 セッションと CSRF

- セッション ID は 32B CSPRNG → hex 64 文字
- Cookie は `HttpOnly`、`SameSite=Lax`、`Secure` (本番)、`Path=/`
- 期限は 24h (活動で延長、上限 7d)
- CSRF はダブルサブミット Cookie とサーバ保存 (sessions.csrf_token) を併用し、HTMX 全 POST/DELETE で `X-CSRF-Token` ヘッダ送信

### 9.7 SQL Sprintf 厳禁の実装担保

- `golangci-lint` の `gocritic`、`gosec`、`sqlclosecheck` を有効化
- `staticcheck` の SA1000 を有効化
- `scripts/hooks/check_no_sql_sprintf.sh` で `grep -RnE 'fmt\.Sprintf\([^)]*"(SELECT|INSERT|UPDATE|DELETE)'` を実行
- pre-commit と CI の quality-gate.sh から呼び出す

## 10. SRS アルゴリズム (SM-2 風)

`internal/domain/srs/sm2.go` に純関数として実装し、`Clock` を引数で注入します。

```go
type Grade int // 0..5 (0=完全な誤答, 5=完璧)

type State struct {
    EFactor      float64
    IntervalDays int
    Repetitions  int
    DueAt        time.Time
    LastGrade    Grade
}

func Next(s State, g Grade, now time.Time) State {
    if g < 3 {
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
    s.EFactor = math.Max(1.3, s.EFactor+(0.1-float64(5-g)*(0.08+float64(5-g)*0.02)))
    s.DueAt = now.AddDate(0, 0, s.IntervalDays)
    s.LastGrade = g
    return s
}
```

弱点重視の上乗せは NextQuestion で以下の比率で抽選します。

- `due_at <= now` を 70%
- 過去 7 日の誤答率上位論点を 20%
- 未着手問題を 10%

Grade は 2 値正誤から以下にマッピングします。

- 正解で 5 秒未満: 5
- 正解で 5〜15 秒: 4
- 正解で 15 秒以上: 3
- 不正解: 0

## 11. UI 構成 (和モダン)

### 11.1 デザイントークン

```css
:root {
  --color-washi: oklch(96% 0.012 80);
  --color-sumi: oklch(20% 0.01 270);
  --color-shu: oklch(55% 0.18 30);
  --color-asagi: oklch(60% 0.08 220);
  --color-kincha: oklch(70% 0.10 80);
  --color-surface: oklch(98% 0.005 80);
  --color-line: oklch(85% 0.005 80);

  --font-mincho: "Noto Serif JP", "Hiragino Mincho ProN", serif;
  --font-gothic: "Noto Sans JP", "Hiragino Sans", sans-serif;

  --text-base: clamp(1rem, 0.92rem + 0.4vw, 1.125rem);
  --text-h1: clamp(1.8rem, 1rem + 3vw, 2.8rem);
  --space-section: clamp(2rem, 1.2rem + 2vw, 4rem);
}
```

### 11.2 主要画面

| 画面 | 構成 |
|---|---|
| ログイン | 和紙背景、中央の縦組み「簿記三級」、半透明セル、セリフ和文 |
| 登録 | ログインと同形式、規約同意チェック |
| ホーム | Bento 4 枚 (今日の正解率/SRS due 件数/直近 7 日 SVG/開始ボタン) |
| 出題 | 大きな問題文 (明朝)、仕訳入力テーブル (借方/貸方)、朱色の検算インジケータ |
| 解説 | 問題再提示、朱の傍点で要点、関連論点リンク |
| 統計 | 論点別正解率 (SVG 棒グラフ)、日次正解率 (SVG 折れ線)、SRS ヒートマップ |
| 履歴 | テーブル、個別削除ボタン、一括削除モーダル |
| 設定 | アカウント、パスワード変更、API トークン |

### 11.3 仕訳入力フォーム

- 行数可変 (追加/削除)
- 勘定科目は HTML 標準の datalist で前方一致補完
- 金額は半角数字 + 3 桁カンマ表示 (Alpine.js が onBlur フォーマット)
- 採点は借方/貸方の (科目, 金額) 集合の一致 (順序不問、空行無視)

## 12. テスト戦略

| 層 | 種類 | 場所 | ツール |
|---|---|---|---|
| domain (SRS、採点) | unit | `internal/domain/**/*_test.go` | testing 標準 |
| service | unit + fakes | `internal/service/**/*_test.go` | port の fake 実装 |
| repo (sqlite) | integration | `internal/repo/sqlite/*_test.go` | 一時 sqlite ファイル |
| handler/middleware | integration | `internal/transport/http/**/*_test.go` | net/http/httptest |
| frontend Alpine | unit | `web/static/js/*.test.mjs` | Node 標準 test runner (`node --test`) |
| E2E | 統合 | `tests/e2e/01-...sh` 〜 `16-...sh` | curl + HTML パターンマッチ |

E2E は外部依存ゼロのため curl ベースで HTML を確認します。Alpine.js の動的挙動は `node --test` でユニット化します。

カバレッジは `go test -cover` の総和で目安 80% とし、CI では fail させません。

## 13. CI と品質ゲート

### 13.1 scripts/quality-gate.sh

```bash
set -euo pipefail

./scripts/hooks/check_gofmt.sh
go vet ./...
staticcheck ./...
golangci-lint run --timeout 5m ./...
govulncheck ./...
./scripts/hooks/check_no_sql_sprintf.sh

go test --count=1 --shuffle=on -race -cover ./...

mkdir -p bin
go build -o bin/boki3-quiz ./cmd/boki3-quiz

if command -v node >/dev/null 2>&1; then
  node --test web/static/js/*.test.mjs
else
  echo "==> node not found; skipping frontend unit tests (CI must install node)"
fi

if [ "${RUN_E2E:-0}" = "1" ]; then
  bash tests/e2e/run-all.sh
fi

gitleaks detect --no-git --source . --redact --no-banner --config .gitleaks.toml
```

### 13.2 .pre-commit-config.yaml

```yaml
repos:
  - repo: local
    hooks:
      - id: quality-gate
        name: quality-gate (gofmt/vet/staticcheck/golangci/govulncheck/test/gitleaks/sql-sprintf-guard)
        entry: bash scripts/quality-gate.sh
        language: system
        pass_filenames: false
        stages: [pre-commit]
```

### 13.3 .github/workflows/ci.yml

go-llm-agent と同形で `pre-commit` ジョブと `e2e` ジョブを分割し、以下をセットアップします。

- Go 1.25 (`actions/setup-go@v6`)
- Python 3.12 (`actions/setup-python@v5`、pre-commit 実行用)
- Node 22 (`actions/setup-node@v4`、`node --test` 実行用)
- staticcheck (`go install honnef.co/go/tools/cmd/staticcheck@latest`)
- golangci-lint v2.12.2 (`golangci/golangci-lint-action@v9`)
- govulncheck v1.3.0
- gitleaks 8.30.1

`e2e` ジョブは `RUN_E2E=1` で `bash scripts/quality-gate.sh` を呼びます。

### 13.4 ローカルレビュー

実装完了後に `coderabbit` をローカル実行し、指摘がなくなるまで自律修正を繰り返します (writing-plans の最終ステップに組込み済み)。

## 14. 設定 (環境変数)

| 変数 | 既定 | 説明 |
|---|---|---|
| `BOKI3_LISTEN` | `:8080` | リッスンアドレス |
| `BOKI3_DB_PATH` | `./data/boki3.db` | SQLite パス |
| `BOKI3_JWT_SECRET` | (必須) | HS256 シークレット (32B 以上) |
| `BOKI3_COOKIE_SECURE` | `false` | 本番では `true` |
| `BOKI3_CORS_ORIGINS` | (空) | allowlist (カンマ区切り) |
| `BOKI3_LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error` |
| `BOKI3_ENV` | `dev` | `dev`/`prod` |

`.env.example` をマスク値でリポジトリに同梱し、`README` に `.env` への cp 手順を明記します。

## 15. 想定外/制約

- SQLite は単一プロセス想定。複数プロセスの水平スケールは対象外
- レートリミットは in-memory のためプロセス再起動でリセット
- 問題の正誤判定は字面の集合一致。意味的な等価性 (例: 「現金」と「現金預金」) は許容しない
- 問題は管理者シードのみ。ユーザーによる問題追加機能は対象外

## 16. 参考リンク

- 商工会議所 簿記 出題区分表: https://www.kentei.ne.jp/bookkeeping/exam-list
- 商業簿記・会計学 (1〜3 級) 2022 年度版区分表 PDF: https://www.kentei.ne.jp/wp/wp-content/uploads/2024/12/shogyouboki_kubun.pdf
- 商業簿記標準・許容勘定科目表 (2〜3 級): https://www.kentei.ne.jp/wp/wp-content/uploads/2021/12/2022_kamoku.pdf
- 簿記 3 級: https://www.kentei.ne.jp/bookkeeping/class3
- 参考 CI: https://github.com/okamyuji/go-llm-agent/blob/main/.github/workflows/ci.yml
- 参考 pre-commit: https://github.com/okamyuji/go-llm-agent/blob/main/.pre-commit-config.yaml
- 参考 quality-gate.sh: https://github.com/okamyuji/go-llm-agent/blob/main/scripts/quality-gate.sh
