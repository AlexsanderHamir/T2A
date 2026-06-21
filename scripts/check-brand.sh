#!/usr/bin/env bash
# Hamix brand guard — fails if retired product identifiers appear outside the allowlist.
#
# Usage (repo root): ./scripts/check-brand.sh

set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"

allowlist_file="$(dirname "$0")/check-brand-allowlist.txt"
fail=0

is_allowlisted() {
  local file="$1"
  local prefix
  while IFS= read -r prefix || [[ -n "$prefix" ]]; do
    [[ -z "$prefix" || "$prefix" =~ ^[[:space:]]*# ]] && continue
    prefix="${prefix//\\//}"
    if [[ "$file" == "$prefix"* ]]; then
      return 0
    fi
  done < "$allowlist_file"
  return 1
}

filter_hits() {
  local hits="$1"
  local line file
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    file="${line%%:*}"
    file="${file#./}"
    if is_allowlisted "$file"; then
      continue
    fi
    echo "$line"
  done <<< "$hits"
}

check_pattern() {
  local label="$1"
  local pattern="$2"
  shift 2
  local hits filtered
  hits="$(rg -n "$pattern" "$@" 2>/dev/null || true)"
  filtered="$(filter_hits "$hits")"
  if [[ -n "$filtered" ]]; then
    echo "check-brand FAILED: $label" >&2
    echo "$filtered" >&2
    fail=1
  fi
}

check_pattern 'retired product word' '\bT2A\b' \
  --glob '!docs/adr/**' \
  --glob '!*.png' \
  --glob '!scripts/check-brand*' \
  --glob '!scripts/check-brand-allowlist.txt'

check_pattern 'retired env prefix' 'T2A_' \
  --glob '!docs/adr/**' \
  --glob '!scripts/check-brand*'

check_pattern 'retired Go module path' 'github.com/AlexsanderHamir/T2A' \
  --glob '!scripts/check-brand*'

check_pattern 'retired worker scratch dir' '\bt2a-worker\b' \
  --glob '!docs/adr/**'

check_pattern 'retired Prometheus namespace' 'Namespace: "t2a"' \
  --glob '!scripts/check-brand*'

check_pattern 'retired npm package name' '\bt2a-web\b'

check_pattern 'retired localStorage prefix' '\bt2a:' \
  --glob '!docs/adr/**'

check_pattern 'retired localStorage key' 't2a_ui_test_mode' \
  --glob '!scripts/check-brand*'

check_pattern 'retired check temp prefix' 't2a-check' \
  --glob '!scripts/check-brand*'

if [[ "$fail" -ne 0 ]]; then
  echo "" >&2
  echo "See docs/naming.md and scripts/check-brand-allowlist.txt" >&2
  exit 1
fi

echo "check-brand OK"
