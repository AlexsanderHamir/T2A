#!/usr/bin/env bash
# taskapi + Vite from repo root: ./scripts/dev.sh  (needs .env / DATABASE_URL)
# Schema migrate is a separate step: ./scripts/migrate.sh
#
# Usage: ./scripts/dev.sh [--migrate] [--host HOST] [--vite-host HOST]
#
# Flags:
#   --migrate         Run ./scripts/migrate.sh first (convenience sugar)
#   --host HOST       taskapi listen host (taskapi -host; default 127.0.0.1)
#   --vite-host HOST  Vite dev server --host (default: localhost only)
#   --help, -h        Show options
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ENV_FILE="$ROOT/.env"
if [[ ! -f "$ENV_FILE" ]]; then
  echo ".env not found at: $ENV_FILE" >&2
  echo "Copy .env.example to .env and set DATABASE_URL:" >&2
  echo "  cp .env.example .env" >&2
  echo "See CONTRIBUTING.md for setup." >&2
  exit 1
fi

HOST=""
VITE_HOST=""
RUN_MIGRATE=0

show_help() {
  sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --migrate)
      RUN_MIGRATE=1
      shift
      ;;
    --host)
      HOST="$2"
      shift 2
      ;;
    --vite-host)
      VITE_HOST="$2"
      shift 2
      ;;
    --help|-h)
      show_help
      exit 0
      ;;
    *)
      echo "unknown flag: $1 (try --help)" >&2
      exit 2
      ;;
  esac
done

GOOS="$(go env GOOS)"
PORT="${DEV_TASKAPI_PORT:-8080}"
EXE="$ROOT/taskapi-dev"
[ "$GOOS" = "windows" ] && EXE="$ROOT/taskapi-dev.exe"

readiness_timeout_sec() {
  local sec
  sec="$(go run ./cmd/devconfig -readiness-timeout-sec 2>/dev/null || echo 150)"
  if [[ "$RUN_MIGRATE" -eq 0 ]]; then
    echo 30
  else
    echo "$sec"
  fi
}

stop_listener_on_port() {
  local port="$1"
  if [ "$GOOS" = "windows" ]; then
    powershell.exe -NoProfile -Command "\$ErrorActionPreference='SilentlyContinue'; Get-NetTCPConnection -LocalPort $port -State Listen | ForEach-Object { if (\$_.OwningProcess) { Stop-Process -Id \$_.OwningProcess -Force -ErrorAction SilentlyContinue } }"
  elif command -v lsof >/dev/null 2>&1; then
    for pid in $(lsof -ti -sTCP:LISTEN -iTCP:"$port" 2>/dev/null || true); do
      kill -9 "$pid" 2>/dev/null || true
    done
  elif command -v fuser >/dev/null 2>&1; then
    fuser -k "${port}/tcp" 2>/dev/null || true
  fi
  sleep 0.2
}

if [[ "$RUN_MIGRATE" -eq 1 ]]; then
  "$ROOT/scripts/migrate.sh"
fi

go mod download
( cd "$ROOT/web" && npm install )
go build -o "$EXE" ./cmd/taskapi

API_PID=""
cleanup() {
  if [ -n "${API_PID:-}" ] && kill -0 "$API_PID" 2>/dev/null; then
    kill "$API_PID" 2>/dev/null || true
    wait "$API_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

api_args=(-port "$PORT")
if [[ -n "$HOST" ]]; then
  api_args=(-host "$HOST" -port "$PORT")
fi

READINESS_SEC="$(readiness_timeout_sec)"
stop_listener_on_port "$PORT"
"$EXE" "${api_args[@]}" &
API_PID=$!

deadline=$((SECONDS + READINESS_SEC))
ready=0
while (( SECONDS < deadline )); do
  kill -0 "$API_PID" 2>/dev/null || break
  if command -v nc >/dev/null 2>&1 && nc -z 127.0.0.1 "$PORT" 2>/dev/null; then
    ready=1
    break
  fi
  if (echo >/dev/tcp/127.0.0.1/"$PORT") 2>/dev/null; then
    ready=1
    break
  fi
  sleep 0.15
done

if (( ready == 1 )) && kill -0 "$API_PID" 2>/dev/null; then
  cd "$ROOT/web"
  if [[ -n "$VITE_HOST" ]]; then
    npm run dev -- --host "$VITE_HOST"
  else
    npm run dev
  fi
  exit 0
fi

if kill -0 "$API_PID" 2>/dev/null; then
  kill "$API_PID" 2>/dev/null || true
  wait "$API_PID" 2>/dev/null || true
  echo "taskapi did not listen on :$PORT within ${READINESS_SEC}s. Try ./scripts/migrate.sh if schema changed." >&2
  exit 1
fi

echo "taskapi exited on :$PORT before listening. See stderr above, logs/taskapi-*.jsonl, or run ./scripts/migrate.sh if schema changed." >&2
exit 1
