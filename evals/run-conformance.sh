#!/usr/bin/env bash
# Boot one target backend, run the feature's hurl suite against it, and
# capture the result to reports/<lang>.log.
#
# Usage: evals/run-conformance.sh <feature> <lang> [port]
#   feature — directory under examples/ and evals/ (e.g. auth)
#   lang    — target under examples/<feature>/targets/ (e.g. go, rust)
#   port    — HTTP port for the backend (default 8080). Refuses to run if
#             something is already listening there, so a stray server can
#             never be mistaken for the target under test.
#
# Env: BOOT_TIMEOUT — seconds to wait for the port (default 180).
#
# Exits with hurl's exit code. The server is always killed on exit. Temp
# files are removed on success; on failure the server log path is printed.
set -euo pipefail

if [ $# -lt 2 ] || [ $# -gt 3 ]; then
  echo "usage: $0 <feature> <lang> [port]" >&2
  exit 2
fi

FEATURE="$1"
LANG_TARGET="$2"
PORT="${3:-8080}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_SH="$ROOT/examples/$FEATURE/targets/$LANG_TARGET/scripts/run.sh"
HURL_FILE="$ROOT/evals/$FEATURE/$FEATURE.hurl"
FIXTURES="$ROOT/evals/$FEATURE/fixtures.env"
REPORT="$ROOT/reports/$LANG_TARGET.log"
BASE_URL="http://localhost:$PORT"
BOOT_TIMEOUT="${BOOT_TIMEOUT:-180}"

for f in "$RUN_SH" "$HURL_FILE" "$FIXTURES"; do
  if [ ! -f "$f" ]; then
    echo "error: missing $f" >&2
    exit 1
  fi
done
command -v hurl >/dev/null || { echo "error: hurl not found on PATH" >&2; exit 1; }

if curl -s -o /dev/null "$BASE_URL/"; then
  echo "error: something is already listening on $BASE_URL; pick another port" >&2
  exit 1
fi

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/candy-$FEATURE-$LANG_TARGET-XXXXXX")"
DB_PATH="$WORK_DIR/$FEATURE.db"
SERVER_LOG="$WORK_DIR/server.log"
SERVER_PID=""
STATUS=1

cleanup() {
  if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
    # run.sh exec's the real binary for Rust, but `go run` forks a child;
    # kill the whole process group so nothing keeps the port.
    kill -- -"$SERVER_PID" 2>/dev/null || kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  if [ "$STATUS" -eq 0 ]; then
    rm -rf "$WORK_DIR"
  else
    rm -f "$DB_PATH" "$DB_PATH-journal" "$DB_PATH-wal" "$DB_PATH-shm"
    echo "==> server log kept at $SERVER_LOG" >&2
  fi
}
trap cleanup EXIT

echo "==> starting $LANG_TARGET backend on $BASE_URL"
# Put the server in its own process group so cleanup can kill it and any
# children. setsid is util-linux; fall back to job control elsewhere.
if command -v setsid >/dev/null; then
  PORT="$PORT" DB_PATH="$DB_PATH" setsid "$RUN_SH" >"$SERVER_LOG" 2>&1 &
else
  set -m
  PORT="$PORT" DB_PATH="$DB_PATH" "$RUN_SH" >"$SERVER_LOG" 2>&1 &
  set +m
fi
SERVER_PID=$!

echo "==> waiting up to ${BOOT_TIMEOUT}s for $BASE_URL"
elapsed=0
until curl -s -o /dev/null "$BASE_URL/"; do
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "error: backend exited before listening; last lines of $SERVER_LOG:" >&2
    tail -n 20 "$SERVER_LOG" >&2
    exit 1
  fi
  if [ "$elapsed" -ge "$BOOT_TIMEOUT" ]; then
    echo "error: backend did not listen on $BASE_URL within ${BOOT_TIMEOUT}s" >&2
    tail -n 20 "$SERVER_LOG" >&2
    exit 1
  fi
  sleep 1
  elapsed=$((elapsed + 1))
done

mkdir -p "$ROOT/reports"
HURL_VERSION="$(hurl --version 2>/dev/null | head -n 1 | awk '{print $2}')"
echo "# hurl $HURL_VERSION — target: examples/$FEATURE/targets/$LANG_TARGET (BASE_URL=$BASE_URL)" >"$REPORT"

# hurl --test prints its per-file results and summary on stderr, so both
# streams go to the report.
set +e
(cd "$ROOT" && hurl --test --no-color \
  --variables-file "evals/$FEATURE/fixtures.env" \
  --variable "BASE_URL=$BASE_URL" \
  "evals/$FEATURE/$FEATURE.hurl") 2>&1 | tee -a "$REPORT"
STATUS=${PIPESTATUS[0]}
set -e

exit "$STATUS"
