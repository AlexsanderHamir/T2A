#!/usr/bin/env bash
# Full local verification: gofmt (check), go vet, go test, funclogmeasure -enforce, web npm test + build.
# Usage from repo root: ./scripts/check.sh
# Skip web: CHECK_SKIP_WEB=1 ./scripts/check.sh
# Skip per-function slog audit: CHECK_SKIP_FUNCLOG=1 ./scripts/check.sh
set -euo pipefail
repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"

echo "gofmt (check)..."
unformatted="$(gofmt -l .)"
if [[ -n "$unformatted" ]]; then
  echo "These files need gofmt:" >&2
  echo "$unformatted" >&2
  exit 1
fi

echo "go vet..."
go vet ./...

echo "go test..."
go test ./... -count=1

if [[ "${CHECK_SKIP_FUNCLOG:-}" != "1" ]]; then
  echo "funclogmeasure (-enforce)..."
  go run ./cmd/funclogmeasure -enforce
fi

if [[ "${CHECK_SKIP_WEB:-}" == "1" ]]; then
  echo "check OK (web skipped)"
  exit 0
fi

if [[ -f web/package.json ]]; then
  echo "web: npm test..."
  (cd web && npm test -- --run && npm run build)
fi

echo "check OK"
