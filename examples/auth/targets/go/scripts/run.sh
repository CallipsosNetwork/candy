#!/usr/bin/env bash
# Run the auth backend. Environment variables:
#   PORT       — HTTP port (default 8080)
#   DB_PATH    — SQLite database path (default /tmp/auth-dev.db)
#   JWT_SECRET — JWT signing secret (default dev-only value)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."
export PORT="${PORT:-8080}"
export DB_PATH="${DB_PATH:-/tmp/auth-dev.db}"
export JWT_SECRET="${JWT_SECRET:-dev-secret-change-in-production}"
exec /usr/local/go/bin/go run ./cmd/server
