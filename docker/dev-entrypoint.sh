#!/usr/bin/env bash
# Docker dev container entrypoint: require DATABASE_URL, then exec command.
# Schema migrate runs inside taskapi on startup (same as native dev.ps1 / dev.sh).
set -euo pipefail

cd /app

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "DATABASE_URL is not set. Copy .env.example to .env and set DATABASE_URL." >&2
  echo "See docs/docker.md if Postgres runs on the host (use host.docker.internal)." >&2
  exit 1
fi

exec "$@"
