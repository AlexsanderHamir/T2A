#!/usr/bin/env bash
# Observability / coverage measurement for the whole Go module.
# Runs tests with a merged cover profile for ./... (every package, including cmd/*).
# go tool cover -func lists only production .go files (not *_test.go) that appear in the profile.
# Resolves the repo root from this script's path (cwd does not matter).
# Usage: ./scripts/measure-observability.sh
# Writes coverage-observability.out under the repo root (gitignored via coverage*.out).
set -euo pipefail
repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"
echo "repo: $repo"

prof="$repo/coverage-observability.out"
go test ./... -coverprofile="$prof" -count=1

cover_func="$(go tool cover -func="$prof")"

echo ""
echo "Per-function slog presence (static check, not coverage): go run ./cmd/funclogmeasure  (or scripts/measure-func-slog.*)"
echo ""
echo "=== Per-function test coverage (all production .go files in this profile) ==="
echo "$cover_func"

echo ""
echo "Profile: $prof (gitignored)"
