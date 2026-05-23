# boki3-quiz-go-htmx 実装ロードマップ

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 日商簿記 3 級学習 Web アプリ boki3-quiz-go-htmx を 9 フェーズに分割して機械的に実装する。

**Architecture:** Go 1.25 + SQLite + HTMX + Alpine.js による単一バイナリ。全層 interface 経由で接続し、Web UI は Cookie セッション、API は JWT Bearer の二重認証。SM-2 風 SRS + 弱点重視で次問題選定。

**Tech Stack:** Go 1.25 / modernc.org/sqlite / golang.org/x/crypto/scrypt / html/template / embed / HTMX / Alpine.js / Node 22 (test runner) / staticcheck / golangci-lint v2.12.2 / govulncheck / gitleaks 8.30.1

**Spec:** `docs/superpowers/specs/2026-05-24-boki3-quiz-design.md`

---

## フェーズ分割

各フェーズは独立してテスト・コミット可能で、次フェーズが前提とする最小限の成果物を残します。

| # | フェーズ | 主な成果物 | 計画書 |
|---|---|---|---|
| 1 | スキャフォールド + 品質ゲート | go.mod / scripts/quality-gate.sh / pre-commit / CI / 最小 main / 最小テスト | `2026-05-24-boki3-quiz-phase1-scaffold.md` |
| 2 | ドメイン層 + SRS | `internal/domain/**` 純粋ロジック、SM-2 関数、採点関数、Grade マッピング、unit test | phase2 計画書 (フェーズ 1 完了後に作成) |
| 3 | SQLite Repository + マイグレーション | migrations/*.sql、`internal/repo/sqlite/*`、port interface、統合テスト (一時 sqlite) | phase3 計画書 |
| 4 | Service 層 + 認証 + JWT 自作 | scrypt hasher、HS256 JWT 自作、AuthService、APITokenService、QuizService、StatsService | phase4 計画書 |
| 5 | HTTP transport (middleware + router) | recover、request_id、access_log、body_limit、security_headers、cors、ratelimit、session、jwt、csrf | phase5 計画書 |
| 6 | Handler + View (html/template + SVG) | 全エンドポイント、html/template、SVG ジェネレータ、エラー画面 | phase6 計画書 |
| 7 | フロントエンド (CSS + Alpine.js + テンプレート) | 和モダンデザイントークン、Alpine コンポーネント、HTMX 連携、Noto Serif/Sans JP セルフホスト | phase7 計画書 |
| 8 | 問題シード (約 430 問) + 3 セット定義 | `internal/data/seed/*.json`、`question_sets`、`question_set_members`、integrity.json | phase8 計画書 |
| 9 | E2E テスト + coderabbit ローカルレビュー | tests/e2e/01..16、`scripts/coderabbit-local.sh`、レビュー指摘反映ループ | phase9 計画書 |

---

## フェーズ 1 へ

フェーズ 1 の詳細計画は `2026-05-24-boki3-quiz-phase1-scaffold.md` を参照してください。フェーズ 1 が完了した時点で `bash scripts/quality-gate.sh` がすべて Pass し、空のサーバが 8080 でヘルスチェックに応答する状態を目指します。
