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
  echo "  -> ? プレースホルダと sql.DB.Exec/Query に置き換えてください" >&2
  exit 1
fi
echo "no-sql-sprintf OK"
