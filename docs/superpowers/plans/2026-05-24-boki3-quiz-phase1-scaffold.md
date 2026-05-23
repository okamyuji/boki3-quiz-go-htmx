# boki3-quiz-go-htmx Phase 1: Scaffold + Quality Gate 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** リポジトリ構造を作成し go-llm-agent と同形の品質ゲート (gofmt/vet/staticcheck/golangci-lint/govulncheck/test/gitleaks/sql-sprintf-guard) をすべて Pass させ、`/healthz` だけ応答する最小バイナリを完成させる。

**Architecture:** Go 1.25 / `database/sql` + `modernc.org/sqlite` を予約 import (フェーズ 3 で使用) / 最小 main + 最小ハンドラ / scripts/quality-gate.sh を pre-commit と CI で共有

**Tech Stack:** Go 1.25, staticcheck latest, golangci-lint v2.12.2, govulncheck v1.3.0, gitleaks 8.30.1, pre-commit 4.x, Python 3.12 (pre-commit 実行用), Node 22 (フェーズ 7 で使用)

**Spec:** `docs/superpowers/specs/2026-05-24-boki3-quiz-design.md`

**Roadmap:** `docs/superpowers/plans/2026-05-24-boki3-quiz-roadmap.md`

---

## File Structure (Phase 1 終了時の状態)

```
boki3-quiz-go-htmx/
├── .github/
│   └── workflows/
│       └── ci.yml
├── .gitignore
├── .gitleaks.toml
├── .golangci.yml
├── .pre-commit-config.yaml
├── .env.example
├── LICENSE
├── Makefile
├── README.md
├── cmd/
│   └── boki3-quiz/
│       ├── main.go
│       └── main_test.go
├── docs/
│   └── superpowers/
│       ├── specs/2026-05-24-boki3-quiz-design.md
│       └── plans/
│           ├── 2026-05-24-boki3-quiz-roadmap.md
│           └── 2026-05-24-boki3-quiz-phase1-scaffold.md
├── go.mod
├── go.sum
├── internal/
│   ├── domain/.keep
│   ├── port/.keep
│   ├── repo/sqlite/.keep
│   ├── service/.keep
│   ├── transport/http/.keep
│   ├── pkg/
│   │   └── version/
│   │       └── version.go
│   └── data/seed/.keep
├── migrations/.keep
├── scripts/
│   ├── hooks/
│   │   ├── check_gofmt.sh
│   │   └── check_no_sql_sprintf.sh
│   ├── quality-gate.sh
│   └── verify-hardening.sh
├── tests/
│   └── e2e/.keep
└── web/
    ├── static/.keep
    └── templates/.keep
```

各ディレクトリ責務:
- `cmd/boki3-quiz/`: バイナリエントリポイント
- `internal/pkg/version/`: ビルド時に埋め込むバージョン情報を提供する小さい責務
- `scripts/`: 品質ゲート、フォーマット検査、SQL Sprintf 防止
- `migrations/`: 後フェーズで使う SQL マイグレーション
- `tests/e2e/`: 後フェーズで使う E2E
- `.keep` は空ディレクトリを git に残すための慣行ファイル

---

### Task 1: ベースディレクトリ構造の作成

**Files:**
- Create: `boki3-quiz-go-htmx/.gitignore`
- Create: `boki3-quiz-go-htmx/LICENSE`
- Create: 上記 File Structure の `.keep` ファイル群

- [ ] **Step 1: 作業ディレクトリへ移動して空構造を作る**

```bash
cd /Users/yujiokamoto/devs/golang/boki3-quiz-go-htmx
mkdir -p cmd/boki3-quiz \
  internal/domain internal/port internal/repo/sqlite \
  internal/service internal/transport/http internal/pkg/version \
  internal/data/seed migrations tests/e2e \
  scripts/hooks web/static web/templates \
  .github/workflows
for d in internal/domain internal/port internal/repo/sqlite \
  internal/service internal/transport/http internal/data/seed \
  migrations tests/e2e web/static web/templates; do
  touch "$d/.keep"
done
```

- [ ] **Step 2: `.gitignore` を作成**

`.gitignore` の内容:
```
# Go
/bin/
*.test
*.out

# OS
.DS_Store

# Editor
.vscode/
.idea/

# Local config / secrets
.env
.env.local
config.yaml
data/
*.db
*.db-wal
*.db-shm

# Build artifacts
coverage.txt
coverage.html
```

- [ ] **Step 3: LICENSE (MIT) を作成**

`LICENSE` の内容:
```
MIT License

Copyright (c) 2026 okamyuji

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 4: git 初期化と最初のコミット**

```bash
cd /Users/yujiokamoto/devs/golang/boki3-quiz-go-htmx
git init
git add .
git commit -m "chore: scaffold project structure"
```

Expected: 「Initialized empty Git repository」と新規コミットが作成される。

---

### Task 2: `go.mod` の初期化

**Files:**
- Create: `go.mod`

- [ ] **Step 1: モジュール初期化**

```bash
cd /Users/yujiokamoto/devs/golang/boki3-quiz-go-htmx
go mod init github.com/okamyuji/boki3-quiz-go-htmx
```

- [ ] **Step 2: `go.mod` の go directive を 1.25 に固定**

`go.mod` の内容を以下に上書きする (`go mod init` の結果に上書き):
```go
module github.com/okamyuji/boki3-quiz-go-htmx

go 1.25
```

理由: CI の `actions/setup-go@v6` の `go-version: "1.25"` と一致させる ([[reference-gomod-directive-ci-match]])

- [ ] **Step 3: コミット**

```bash
git add go.mod
git commit -m "chore: initialize go module with go 1.25"
```

---

### Task 3: `internal/pkg/version/version.go` を作成 (最小実装と最小テスト)

**Files:**
- Create: `internal/pkg/version/version.go`
- Test: `internal/pkg/version/version_test.go`

- [ ] **Step 1: テストを書く (RED)**

`internal/pkg/version/version_test.go`:
```go
package version_test

import (
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/version"
)

func TestStringNonEmpty(t *testing.T) {
	t.Parallel()
	if got := version.String(); got == "" {
		t.Fatalf("version.String() = %q, want non-empty", got)
	}
}

func TestStringDefault(t *testing.T) {
	t.Parallel()
	if got := version.String(); got != "dev" {
		t.Fatalf("version.String() = %q, want %q", got, "dev")
	}
}
```

- [ ] **Step 2: 失敗確認**

```bash
go test ./internal/pkg/version/...
```

Expected: ビルドエラー (package が無いため)

- [ ] **Step 3: 最小実装**

`internal/pkg/version/version.go`:
```go
// Package version exposes the application build version.
// The value is overridden at link time with -ldflags "-X .../version.value=v1.2.3".
package version

var value = "dev"

// String returns the build version.
func String() string {
	return value
}
```

- [ ] **Step 4: 成功確認**

```bash
go test -count=1 -race ./internal/pkg/version/...
```

Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/pkg/version/
git commit -m "feat(version): introduce build version package"
```

---

### Task 4: 最小 main + `/healthz` ハンドラ + 統合テスト

**Files:**
- Create: `cmd/boki3-quiz/main.go`
- Test: `cmd/boki3-quiz/main_test.go`

- [ ] **Step 1: テストを書く (RED)**

`cmd/boki3-quiz/main_test.go`:
```go
package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthzReturnsOK(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newRouter())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	t.Cleanup(func() {
		if cerr := resp.Body.Close(); cerr != nil {
			t.Logf("close body: %v", cerr)
		}
	})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if got := strings.TrimSpace(string(body)); got != "ok" {
		t.Fatalf("body = %q, want %q", got, "ok")
	}
}

func TestVersionReturnsBuildVersion(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newRouter())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/version")
	if err != nil {
		t.Fatalf("GET /version: %v", err)
	}
	t.Cleanup(func() {
		if cerr := resp.Body.Close(); cerr != nil {
			t.Logf("close body: %v", cerr)
		}
	})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if got := strings.TrimSpace(string(body)); got == "" {
		t.Fatalf("body is empty")
	}
}
```

- [ ] **Step 2: 失敗確認**

```bash
go test ./cmd/boki3-quiz/...
```

Expected: `newRouter` が未定義のためビルドエラー

- [ ] **Step 3: 最小実装**

`cmd/boki3-quiz/main.go`:
```go
// Package main is the entrypoint of the boki3-quiz HTTP server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/version"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server terminated", "err", err)
		os.Exit(1)
	}
}

func run() error {
	addr := os.Getenv("BOKI3_LISTEN")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           newRouter(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting", "addr", addr, "version", version.String())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func newRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthz)
	mux.HandleFunc("/readyz", healthz)
	mux.HandleFunc("/version", versionHandler)
	return mux
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok\n")); err != nil {
		slog.Warn("write healthz body", "err", err)
	}
}

func versionHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(version.String() + "\n")); err != nil {
		slog.Warn("write version body", "err", err)
	}
}
```

- [ ] **Step 4: 成功確認**

```bash
go test -count=1 -race ./cmd/boki3-quiz/...
```

Expected: 2 テストとも PASS

- [ ] **Step 5: コミット**

```bash
git add cmd/boki3-quiz/
git commit -m "feat(server): minimal http server with healthz, readyz, version"
```

---

### Task 5: `scripts/hooks/check_gofmt.sh` を作成

**Files:**
- Create: `scripts/hooks/check_gofmt.sh`

- [ ] **Step 1: スクリプトを書く**

`scripts/hooks/check_gofmt.sh`:
```bash
#!/usr/bin/env bash
# check_gofmt.sh
# gofmt 未整形のファイルがあれば一覧して非 0 終了する。
set -euo pipefail

diff_files=$(gofmt -l .)
if [ -n "$diff_files" ]; then
  echo "ERROR: gofmt 未整形のファイル:" >&2
  echo "$diff_files" >&2
  echo "  → 'gofmt -w .' を実行してください" >&2
  exit 1
fi
echo "gofmt OK"
```

- [ ] **Step 2: 実行ビット付与**

```bash
chmod +x scripts/hooks/check_gofmt.sh
```

- [ ] **Step 3: 手動実行で確認**

```bash
./scripts/hooks/check_gofmt.sh
```

Expected: `gofmt OK` と表示される。

- [ ] **Step 4: コミット**

```bash
git add scripts/hooks/check_gofmt.sh
git commit -m "chore(quality-gate): add gofmt check hook"
```

---

### Task 6: `scripts/hooks/check_no_sql_sprintf.sh` を作成

**Files:**
- Create: `scripts/hooks/check_no_sql_sprintf.sh`

- [ ] **Step 1: スクリプトを書く**

`scripts/hooks/check_no_sql_sprintf.sh`:
```bash
#!/usr/bin/env bash
# check_no_sql_sprintf.sh
# fmt.Sprintf に SELECT/INSERT/UPDATE/DELETE を含む文字列リテラルを直接埋めると
# SQL インジェクションの温床となる。本リポジトリでは ? プレースホルダのみを許可する。
# 本スクリプトは Go ソースを再帰検索し、違反があれば失敗終了する。
set -euo pipefail

violations=$(grep -RnE --include='*.go' \
  'fmt\.Sprintf\([^)]*"(SELECT|INSERT|UPDATE|DELETE|MERGE)' . \
  || true)

if [ -n "$violations" ]; then
  echo "ERROR: fmt.Sprintf による SQL 文字列連結が検出されました" >&2
  echo "$violations" >&2
  echo "  → ? プレースホルダと sql.DB.Exec/Query に置き換えてください" >&2
  exit 1
fi
echo "no-sql-sprintf OK"
```

- [ ] **Step 2: 実行ビット付与と手動確認**

```bash
chmod +x scripts/hooks/check_no_sql_sprintf.sh
./scripts/hooks/check_no_sql_sprintf.sh
```

Expected: `no-sql-sprintf OK`

- [ ] **Step 3: コミット**

```bash
git add scripts/hooks/check_no_sql_sprintf.sh
git commit -m "chore(quality-gate): add sql-sprintf guard hook"
```

---

### Task 7: `.gitleaks.toml` を作成

**Files:**
- Create: `.gitleaks.toml`

- [ ] **Step 1: ファイル作成**

`.gitleaks.toml`:
```toml
# gitleaks 8.30.x config
# 既定ルールに加え、本リポジトリ固有の許可リストを定義する。
title = "boki3-quiz-go-htmx gitleaks config"

[extend]
useDefault = true

[allowlist]
description = "テスト用ダミー資格情報を gitleaks の検出対象から除外する"
paths = [
  '''(?i)tests/.*''',
  '''(?i)docs/.*''',
]

# テスト用に JWT のサンプルが含まれることを許容する (実シークレットは含めない)
regexes = [
  '''eyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}''',
]
```

- [ ] **Step 2: gitleaks をローカルで実行して確認 (gitleaks が未インストールの場合スキップ)**

```bash
if command -v gitleaks >/dev/null 2>&1; then
  gitleaks detect --no-git --source . --redact --no-banner --config .gitleaks.toml
fi
```

Expected: 検出件数 0 または `gitleaks` 未インストールでスキップ。

- [ ] **Step 3: コミット**

```bash
git add .gitleaks.toml
git commit -m "chore(quality-gate): add gitleaks config"
```

---

### Task 8: `.golangci.yml` を作成

**Files:**
- Create: `.golangci.yml`

- [ ] **Step 1: ファイル作成**

`.golangci.yml` (golangci-lint v2 系):
```yaml
version: "2"

run:
  timeout: 5m
  go: "1.25"

linters:
  default: none
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gosec
    - gocritic
    - revive
    - sqlclosecheck
    - bodyclose
    - copyloopvar
    - misspell
    - prealloc

linters-settings:
  gosec:
    excludes:
      - G104  # errcheck と重複
  gocritic:
    enabled-tags:
      - diagnostic
      - performance
      - style
    disabled-checks:
      - hugeParam   # 受信側のサイズ警告は本プロジェクトでは off

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
  exclude-rules:
    - path: _test\.go
      linters:
        - gosec
        - errcheck
```

- [ ] **Step 2: ローカルで実行 (golangci-lint 未インストールならスキップ)**

```bash
if command -v golangci-lint >/dev/null 2>&1; then
  golangci-lint run --timeout 5m ./...
fi
```

Expected: 検出 0 件、もしくは未インストールでスキップ。違反が出た場合は最小実装に gofmt / コメント追加で対応。

- [ ] **Step 3: コミット**

```bash
git add .golangci.yml
git commit -m "chore(quality-gate): add golangci-lint v2 config"
```

---

### Task 9: `scripts/quality-gate.sh` を作成

**Files:**
- Create: `scripts/quality-gate.sh`

- [ ] **Step 1: スクリプトを書く**

`scripts/quality-gate.sh`:
```bash
#!/usr/bin/env bash
# quality-gate.sh
# pre-commit と CI から同一コマンドで呼び出される品質ゲート。
# - gofmt / go vet / staticcheck / golangci-lint / govulncheck / SQL Sprintf guard
# - go test (count=1 shuffle=on race cover)
# - release build smoke
# - frontend unit (node --test) があれば実行
# - gitleaks
# - RUN_E2E=1 のとき E2E
set -euo pipefail

echo "==> gofmt"
./scripts/hooks/check_gofmt.sh

echo "==> go vet"
go vet ./...

echo "==> staticcheck"
staticcheck ./...

echo "==> golangci-lint"
golangci-lint run --timeout 5m ./...

echo "==> govulncheck"
govulncheck ./...

echo "==> check_no_sql_sprintf"
./scripts/hooks/check_no_sql_sprintf.sh

echo "==> go test (count=1 shuffle=on race cover)"
go test --count=1 --shuffle=on -race -cover ./...

echo "==> release build smoke (go build -o bin/boki3-quiz)"
mkdir -p bin
go build -o bin/boki3-quiz ./cmd/boki3-quiz

echo "==> frontend unit (node --test)"
if command -v node >/dev/null 2>&1; then
  shopt -s nullglob
  files=(web/static/js/*.test.mjs)
  shopt -u nullglob
  if [ "${#files[@]}" -gt 0 ]; then
    node --test "${files[@]}"
  else
    echo "  (no frontend unit tests yet, skipping)"
  fi
else
  echo "  (node not installed, skipping; CI must install node)"
fi

echo "==> staged-secret-files-guard"
staged_sensitive=$(git diff --cached --name-only 2>/dev/null \
  | grep -E '^(\.env|config\.yaml)$' || true)
if [ -n "$staged_sensitive" ]; then
  echo "ERROR: 機密ファイルが staged されています:" >&2
  echo "$staged_sensitive" >&2
  echo "  → git reset HEAD <file>" >&2
  exit 2
fi

echo "==> gitleaks (detect --no-git: scans working tree including staged files)"
if command -v gitleaks >/dev/null 2>&1; then
  gitleaks detect --no-git --source . --redact --no-banner --config .gitleaks.toml
else
  echo "  (gitleaks not installed, skipping; CI installs gitleaks)"
fi

if [ "${RUN_E2E:-0}" = "1" ]; then
  echo "==> e2e (phase 9 で追加)"
  shopt -s nullglob
  e2e_failed=0
  for s in tests/e2e/*.sh; do
    echo "    > $s"
    if ! bash "$s"; then
      e2e_failed=1
      if [ "${RUN_E2E_KEEPGOING:-0}" != "1" ]; then
        exit 1
      fi
    fi
  done
  shopt -u nullglob
  if [ "$e2e_failed" -ne 0 ]; then
    echo "==> one or more e2e scripts failed (keep-going mode)"
    exit 1
  fi
fi

echo "all quality checks passed"
```

- [ ] **Step 2: 実行ビット付与**

```bash
chmod +x scripts/quality-gate.sh
```

- [ ] **Step 3: 必要ツールをインストール (未インストールなら)**

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
go install golang.org/x/vuln/cmd/govulncheck@v1.3.0
# golangci-lint v2.12.2 (Homebrew or バイナリで)
if ! command -v golangci-lint >/dev/null 2>&1; then
  echo "warn: golangci-lint not installed; install before commit"
fi
if ! command -v gitleaks >/dev/null 2>&1; then
  echo "warn: gitleaks not installed; install before commit"
fi
```

- [ ] **Step 4: 手動で quality-gate.sh を試走**

```bash
bash scripts/quality-gate.sh
```

Expected: 各セクションが順に PASS し、最後に `all quality checks passed` と表示。

- [ ] **Step 5: コミット**

```bash
git add scripts/quality-gate.sh
git commit -m "chore(quality-gate): add quality-gate.sh"
```

---

### Task 10: `.pre-commit-config.yaml` を作成

**Files:**
- Create: `.pre-commit-config.yaml`

- [ ] **Step 1: ファイル作成**

`.pre-commit-config.yaml`:
```yaml
# pre-commit と CI は同一の scripts/quality-gate.sh を呼ぶ。
# entry にコロン+スペースを書くと pre-commit 4.x の YAML パーサが mapping value と
# 誤認するため scripts/quality-gate.sh をスクリプトファイル化して呼び出す。
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

- [ ] **Step 2: pre-commit をインストール (未インストールなら)**

```bash
pip install --user --upgrade pre-commit
pre-commit install
```

- [ ] **Step 3: pre-commit run の動作確認**

```bash
pre-commit run --all-files
```

Expected: quality-gate がすべて PASS。

- [ ] **Step 4: コミット**

```bash
git add .pre-commit-config.yaml
git commit -m "chore(quality-gate): add pre-commit config"
```

---

### Task 11: `scripts/verify-hardening.sh` を作成

**Files:**
- Create: `scripts/verify-hardening.sh`

- [ ] **Step 1: スクリプトを書く**

`scripts/verify-hardening.sh`:
```bash
#!/usr/bin/env bash
# verify-hardening.sh
# リポジトリ全体のハードニング設定が揃っているかを CI で確認する。
# Phase 1 では雛形のみ。Phase 5 以降で security headers / cookie 設定の確認を追加する。
set -euo pipefail

required_files=(
  ".gitignore"
  ".gitleaks.toml"
  ".golangci.yml"
  ".pre-commit-config.yaml"
  "scripts/quality-gate.sh"
  "scripts/hooks/check_gofmt.sh"
  "scripts/hooks/check_no_sql_sprintf.sh"
  "go.mod"
)

missing=0
for f in "${required_files[@]}"; do
  if [ ! -f "$f" ]; then
    echo "ERROR: required hardening file missing: $f" >&2
    missing=1
  fi
done
if [ "$missing" -ne 0 ]; then
  exit 1
fi

# scripts は executable 必須
for s in scripts/quality-gate.sh \
         scripts/hooks/check_gofmt.sh \
         scripts/hooks/check_no_sql_sprintf.sh; do
  if [ ! -x "$s" ]; then
    echo "ERROR: not executable: $s" >&2
    exit 1
  fi
done

echo "hardening verification OK"
```

- [ ] **Step 2: 実行ビット付与と動作確認**

```bash
chmod +x scripts/verify-hardening.sh
bash scripts/verify-hardening.sh
```

Expected: `hardening verification OK`

- [ ] **Step 3: コミット**

```bash
git add scripts/verify-hardening.sh
git commit -m "chore(quality-gate): add hardening verification script"
```

---

### Task 12: `.github/workflows/ci.yml` を作成

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: ファイル作成**

`.github/workflows/ci.yml`:
```yaml
# boki3-quiz-go-htmx CI workflow

name: CI

on:
  push:
    branches: ["**"]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  pre-commit:
    name: pre-commit (quality-gate + gitleaks)
    runs-on: ubuntu-latest
    env:
      GITLEAKS_VERSION: "8.30.1"
    steps:
      - uses: actions/checkout@v5
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: "1.25"
          cache: true

      - name: Set up Python
        uses: actions/setup-python@v5
        with:
          python-version: "3.12"

      - name: Set up Node
        uses: actions/setup-node@v4
        with:
          node-version: "22"

      - name: Install staticcheck
        run: go install honnef.co/go/tools/cmd/staticcheck@latest

      - name: Install golangci-lint
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.12.2

      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@v1.3.0

      - name: Install gitleaks
        run: |
          set -euo pipefail
          curl -sSL "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz" \
            | sudo tar -xz -C /usr/local/bin gitleaks

      - name: Install pre-commit
        run: pip install --upgrade pre-commit

      - name: Run pre-commit hooks (品質ゲート + gitleaks)
        run: pre-commit run --all-files --show-diff-on-failure --color always

      - name: Run hardening verification
        run: bash scripts/verify-hardening.sh

  e2e:
    name: e2e (phase 9 以降)
    runs-on: ubuntu-latest
    env:
      GITLEAKS_VERSION: "8.30.1"
    steps:
      - uses: actions/checkout@v5
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: "1.25"
          cache: true

      - name: Set up Node
        uses: actions/setup-node@v4
        with:
          node-version: "22"

      - name: Install staticcheck
        run: go install honnef.co/go/tools/cmd/staticcheck@latest

      - name: Install golangci-lint
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.12.2

      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@v1.3.0

      - name: Install gitleaks
        run: |
          set -euo pipefail
          curl -sSL "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz" \
            | sudo tar -xz -C /usr/local/bin gitleaks

      - name: Run quality gate with E2E
        env:
          RUN_E2E: "1"
        run: bash scripts/quality-gate.sh
```

- [ ] **Step 2: コミット**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add github actions workflow (pre-commit + e2e)"
```

---

### Task 13: `Makefile` の作成

**Files:**
- Create: `Makefile`

- [ ] **Step 1: ファイル作成**

`Makefile`:
```makefile
.PHONY: all build run test lint quality-gate clean fmt

GO ?= go
BIN := bin/boki3-quiz

all: quality-gate

build:
	mkdir -p bin
	$(GO) build -o $(BIN) ./cmd/boki3-quiz

run: build
	./$(BIN)

test:
	$(GO) test --count=1 --shuffle=on -race -cover ./...

lint:
	golangci-lint run --timeout 5m ./...

fmt:
	$(GO) fmt ./...

quality-gate:
	bash scripts/quality-gate.sh

clean:
	rm -rf bin coverage.txt coverage.html
```

- [ ] **Step 2: 動作確認**

```bash
make build
ls -l bin/boki3-quiz
make test
```

Expected: `bin/boki3-quiz` が生成され、`make test` が PASS。

- [ ] **Step 3: コミット**

```bash
git add Makefile
git commit -m "chore: add Makefile"
```

---

### Task 14: `.env.example` の作成

**Files:**
- Create: `.env.example`

- [ ] **Step 1: ファイル作成**

`.env.example`:
```bash
# boki3-quiz-go-htmx 環境変数サンプル
# 利用方法:
#   cp .env.example .env
# .env 自体は .gitignore で除外されています。
# 32B 以上の JWT シークレットを必ず生成して BOKI3_JWT_SECRET に設定してください。

# HTTP リッスンアドレス
BOKI3_LISTEN=:8080

# SQLite ファイルパス
BOKI3_DB_PATH=./data/boki3.db

# JWT HS256 シークレット (32 バイト以上、本番は openssl rand -hex 64 で生成)
BOKI3_JWT_SECRET=REPLACE_WITH_AT_LEAST_32_BYTES_OF_RANDOM_HEX

# Cookie Secure 属性 (本番は true、ローカル開発は false)
BOKI3_COOKIE_SECURE=false

# CORS allowlist (カンマ区切り、空のときは同一オリジンのみ)
BOKI3_CORS_ORIGINS=

# ログレベル (debug/info/warn/error)
BOKI3_LOG_LEVEL=info

# 環境名 (dev/prod)
BOKI3_ENV=dev
```

- [ ] **Step 2: コミット**

```bash
git add .env.example
git commit -m "docs(env): add .env.example with masked values"
```

---

### Task 15: `README.md` の作成

**Files:**
- Create: `README.md`

- [ ] **Step 1: ファイル作成**

`README.md`:
````markdown
# boki3-quiz-go-htmx

日商簿記検定 3 級学習 Web アプリです。Go 1.25 と SQLite、HTMX、Alpine.js のみで実装し、Web UI は Cookie セッション、JSON API は JWT Bearer の二重認証を提供します。

> 出題範囲は商工会議所の公式区分表 (2022 年度版・2026 年度試験適用) に準拠します。
> 参考: https://www.kentei.ne.jp/bookkeeping/exam-list

## 必要環境

- Go 1.25
- (任意) Node 22 (フロントエンド unit test 実行に使用)
- (任意) pre-commit, staticcheck, golangci-lint v2.12.2, govulncheck v1.3.0, gitleaks 8.30.1

## セットアップ

```bash
cp .env.example .env
# .env 内の BOKI3_JWT_SECRET を 32 バイト以上のランダム値に書き換える
make build
make run
```

`/healthz` `/readyz` `/version` がデフォルトで `:8080` で応答します。

## 品質ゲート

pre-commit と CI は同じ `scripts/quality-gate.sh` を呼びます。ローカルでも以下で実行できます。

```bash
make quality-gate
```

gofmt / go vet / staticcheck / golangci-lint / govulncheck / SQL Sprintf guard / go test / リリースビルド / gitleaks / (任意で E2E) を順に実行します。

## ライセンス

MIT (`LICENSE` 参照)
````

- [ ] **Step 2: コミット**

```bash
git add README.md
git commit -m "docs: add README"
```

---

### Task 16: 全体動作確認

- [ ] **Step 1: 品質ゲート全体を実行**

```bash
bash scripts/quality-gate.sh
```

Expected: 各セクションが PASS し、最後に `all quality checks passed`。

- [ ] **Step 2: pre-commit でも実行**

```bash
pre-commit run --all-files
```

Expected: quality-gate hook が PASS。

- [ ] **Step 3: サーバ起動確認**

```bash
./bin/boki3-quiz &
sleep 1
curl -sf http://localhost:8080/healthz
curl -sf http://localhost:8080/version
kill %1
```

Expected: `ok` と `dev` (もしくはビルド版数) が返る。

- [ ] **Step 4: 最終コミット (もし未追跡があれば)**

```bash
git status
```

未追跡が無いことを確認。

---

## Self-Review チェックリスト

実装完了後に以下を自己確認します。

- [ ] `scripts/quality-gate.sh` が PASS する
- [ ] CI ワークフローが pre-commit と e2e の 2 ジョブを持つ
- [ ] `go.mod` の go directive が `1.25` で CI と一致する
- [ ] `.env.example` がマスク値で同梱され `.env` は `.gitignore` で除外されている
- [ ] `LICENSE`、`README.md` が存在する
- [ ] `cmd/boki3-quiz/main_test.go` が PASS する
- [ ] フェーズ 2 以降で使う各 internal ディレクトリが `.keep` で確保されている

## Phase 1 完了条件

1. `bash scripts/quality-gate.sh` が exit 0 で完了する
2. `make build` で `bin/boki3-quiz` が生成される
3. `./bin/boki3-quiz` 起動後 `/healthz` が `ok` を返す
4. すべての変更が git にコミットされている

完了したらフェーズ 2 (ドメイン層 + SRS) の計画書作成へ進みます。
