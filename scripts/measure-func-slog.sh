#!/usr/bin/env bash
# Measures how many named functions/methods contain a direct log/slog call (see docs/OBSERVABILITY.md).
# Resolves repo root from this script path; extra args are passed to funclogmeasure (e.g. -json, -enforce).
set -euo pipefail
repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"
exec go run ./cmd/funclogmeasure "$@"
