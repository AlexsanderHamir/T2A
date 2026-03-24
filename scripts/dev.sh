#!/usr/bin/env bash
# taskapi + Vite from repo root: ./scripts/dev.sh  (needs .env / DATABASE_URL)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
GOOS="$(go env GOOS)"
PORT="${DEV_TASKAPI_PORT:-8080}"
EXE="$ROOT/taskapi-dev"
[ "$GOOS" = "windows" ] && EXE="$ROOT/taskapi-dev.exe"

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

for attempt in 1 2; do
  stop_listener_on_port "$PORT"
  "$EXE" -port "$PORT" &
  API_PID=$!

  deadline=$((SECONDS + 90))
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
    break
  fi

  if kill -0 "$API_PID" 2>/dev/null; then
    kill "$API_PID" 2>/dev/null || true
    wait "$API_PID" 2>/dev/null || true
  fi
  API_PID=""

  if [ "$attempt" = 1 ]; then
    continue
  fi
  echo "taskapi did not start on :$PORT" >&2
  exit 1
done

cd "$ROOT/web"
npm run dev
