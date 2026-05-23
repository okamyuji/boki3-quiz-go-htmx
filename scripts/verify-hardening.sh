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

for s in scripts/quality-gate.sh \
         scripts/hooks/check_gofmt.sh \
         scripts/hooks/check_no_sql_sprintf.sh; do
  if [ ! -x "$s" ]; then
    echo "ERROR: not executable: $s" >&2
    exit 1
  fi
done

echo "hardening verification OK"
