#!/usr/bin/env bash
# check_gofmt.sh
# gofmt 未整形のファイルがあれば一覧して非 0 終了する。
set -euo pipefail

diff_files=$(gofmt -l .)
if [ -n "$diff_files" ]; then
  echo "ERROR: gofmt 未整形のファイル:" >&2
  echo "$diff_files" >&2
  echo "  -> 'gofmt -w .' を実行してください" >&2
  exit 1
fi
echo "gofmt OK"
