#!/usr/bin/env bash
# Start taskapi and the Vite dev server (web/). Stops the API when npm exits (Ctrl+C).
# Runs go mod download and npm install in web/ first so dependencies are ready.
# Requires: Go, Node/npm, repo-root .env with DATABASE_URL for Postgres.
# Usage (from repo root):  ./scripts/dev.sh

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

go mod download
( cd "$ROOT/web" && npm install )

go run ./cmd/taskapi &
API_PID=$!

cleanup() {
  if kill -0 "$API_PID" 2>/dev/null; then
    kill "$API_PID" 2>/dev/null || true
    wait "$API_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

cd "$ROOT/web"
npm run dev
