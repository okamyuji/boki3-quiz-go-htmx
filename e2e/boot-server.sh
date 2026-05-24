#!/usr/bin/env bash
# boot-server.sh — Playwright webServer 用エントリポイント。
#
# - 一時 DB を /tmp/boki3-e2e-<unix>.db に作成
# - seed-loader で 470 問を投入
# - boki3-quiz をフォアグラウンドで起動 (Playwright が SIGTERM で停止)
#
# 環境変数:
#   BOKI3_E2E_PORT (既定 18080)
#   BOKI3_E2E_DB   (既定 /tmp/boki3-e2e-<unix>.db)

set -euo pipefail

# このスクリプトの位置からプロジェクトルートを解決。
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"

PORT="${BOKI3_E2E_PORT:-18080}"
DB="${BOKI3_E2E_DB:-/tmp/boki3-e2e-$(date +%s%N).db}"
SECRET="${BOKI3_JWT_SECRET:-$(openssl rand -hex 32)}"

cd "$ROOT"

# E2E 用 DB を初期化 (毎回 fresh)
rm -f "$DB"
go run ./cmd/seed-loader -db "$DB" -seed seed 1>&2

# サーバ起動 (フォアグラウンド)
# scrypt N=4096 にして E2E を高速化 (本番は 32768)。
exec env \
  BOKI3_LISTEN=":$PORT" \
  BOKI3_DB_PATH="$DB" \
  BOKI3_JWT_SECRET="$SECRET" \
  BOKI3_COOKIE_SECURE=false \
  BOKI3_ENV=dev \
  BOKI3_SCRYPT_N=4096 \
  BOKI3_RL_GLOBAL_MAX=10000 \
  BOKI3_RL_LOGIN_MAX=1000 \
  BOKI3_RL_API_BURST=1000 \
  go run ./cmd/boki3-quiz
