#!/usr/bin/env bash
# dev.sh — start/stop the Go server (:19320) and Vite dev server (:19321) in the background.
#
# Usage:
#   scripts/dev.sh start      # launch both, detached
#   scripts/dev.sh stop       # kill both
#   scripts/dev.sh status     # show pid + listening status
#   scripts/dev.sh logs       # tail both logs
#   scripts/dev.sh restart    # stop then start
#
# Env overrides:
#   BOOKSHELF_DB_PATH       default: ./.dev/bookshelf.db
#
# Note: the library directory is now a runtime setting configured via the
# Settings page in the SPA (PUT /api/settings). It is no longer an env var.

set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$PWD"
DEV_DIR="$ROOT/.dev"
mkdir -p "$DEV_DIR"

DB_PATH="${BOOKSHELF_DB_PATH:-$DEV_DIR/bookshelf.db}"
mkdir -p "$(dirname "$DB_PATH")"

SERVER_PID="$DEV_DIR/server.pid"
VITE_PID="$DEV_DIR/vite.pid"
SERVER_LOG="$DEV_DIR/server.log"
VITE_LOG="$DEV_DIR/vite.log"

is_running() { [[ -f "$1" ]] && kill -0 "$(cat "$1")" 2>/dev/null; }

start_server() {
  if is_running "$SERVER_PID"; then
    echo "go server already running (pid $(cat "$SERVER_PID"))"
    return
  fi
  echo "starting go server on :19320 (db=$DB_PATH)"
  BOOKSHELF_DB_PATH="$DB_PATH" \
  BOOKSHELF_LISTEN=":19320" \
    nohup go run ./cmd/bookshelf >"$SERVER_LOG" 2>&1 &
  echo $! >"$SERVER_PID"
  disown 2>/dev/null || true
}

start_vite() {
  if is_running "$VITE_PID"; then
    echo "vite already running (pid $(cat "$VITE_PID"))"
    return
  fi
  echo "starting vite dev server on :19321"
  ( cd web && nohup npm run dev -- --host >"$VITE_LOG" 2>&1 & echo $! >"$VITE_PID" )
  disown 2>/dev/null || true
}

stop_one() {
  local pidfile="$1" name="$2"
  if is_running "$pidfile"; then
    local pid
    pid="$(cat "$pidfile")"
    echo "stopping $name (pid $pid)"
    kill "$pid" 2>/dev/null || true
    # also kill child processes (go run / npm spawn children)
    pkill -P "$pid" 2>/dev/null || true
    rm -f "$pidfile"
  else
    echo "$name not running"
    rm -f "$pidfile"
  fi
}

status_one() {
  local pidfile="$1" name="$2" port="$3"
  if is_running "$pidfile"; then
    echo "$name: running (pid $(cat "$pidfile"))"
  else
    echo "$name: stopped"
  fi
  if command -v curl >/dev/null 2>&1; then
    if curl -fsS -o /dev/null --max-time 1 "http://localhost:$port/${4:-}"; then
      echo "  http://localhost:$port responding"
    fi
  fi
}

case "${1:-start}" in
  start)
    start_server
    start_vite
    echo
    echo "logs:    tail -f $SERVER_LOG $VITE_LOG"
    echo "app:     http://localhost:19321  (proxies api to :19320)"
    echo "api:     http://localhost:19320"
    ;;
  stop)
    stop_one "$VITE_PID" vite
    stop_one "$SERVER_PID" server
    ;;
  restart)
    "$0" stop
    "$0" start
    ;;
  status)
    status_one "$SERVER_PID" server 19320 healthz
    status_one "$VITE_PID" vite 19321
    ;;
  logs)
    tail -f "$SERVER_LOG" "$VITE_LOG"
    ;;
  *)
    echo "usage: $0 {start|stop|restart|status|logs}" >&2
    exit 2
    ;;
esac
