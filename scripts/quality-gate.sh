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
  echo "  -> git reset HEAD <file>" >&2
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
