#!/usr/bin/env bash
# Fail when a CI test group's statement coverage is below its floor.
# Usage: ./scripts/coverage-gate.sh <group> [--profile=path]
#   --profile=path  Reuse an existing cover profile (skips go test).

set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$script_dir/.." && pwd)"
cd "$repo"

# shellcheck source=test-groups.sh
source "$script_dir/test-groups.sh"

GROUP=""
PROFILE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --profile=*)
      PROFILE="${1#--profile=}"
      shift
      ;;
    --profile)
      PROFILE="${2:-}"
      shift 2
      ;;
    *)
      GROUP="$1"
      shift
      ;;
  esac
done

if [[ -z "$GROUP" ]]; then
  echo "usage: $0 <group> [--profile=path]" >&2
  echo "valid groups: $(group_names)" >&2
  exit 2
fi

if ! group_packages "$GROUP" >/dev/null 2>&1; then
  exit 2
fi

BASELINES="$script_dir/coverage-baselines.json"
if [[ ! -f "$BASELINES" ]]; then
  echo "missing $BASELINES" >&2
  exit 1
fi

floor="$(sed -n "s/.*\"${GROUP}\"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p" "$BASELINES" | head -1)"
if [[ -z "$floor" ]]; then
  echo "no baseline floor for group: $GROUP" >&2
  exit 1
fi

cleanup_profile() {
  if [[ -n "${owned_profile:-}" && -f "$owned_profile" ]]; then
    rm -f "$owned_profile"
  fi
}

owned_profile=""
if [[ -z "$PROFILE" ]]; then
  owned_profile="$(mktemp "${TMPDIR:-/tmp}/hamix-cover.XXXXXX")"
  PROFILE="$owned_profile"
  trap cleanup_profile EXIT

  targets="$(group_packages "$GROUP")"
  set +e
  # shellcheck disable=SC2086
  go test $targets -count=1 -coverprofile="$PROFILE" >/dev/null 2>&1
  code=$?
  set -e
  if [[ $code -ne 0 ]]; then
    echo "${GROUP}: go test failed (exit $code)" >&2
    exit "$code"
  fi
fi

if [[ ! -f "$PROFILE" ]]; then
  echo "cover profile not found: $PROFILE" >&2
  exit 1
fi

total_line="$(go tool cover -func="$PROFILE" | tail -1)"
pct="$(echo "$total_line" | awk '{print $NF}' | tr -d '%')"

if awk "BEGIN { exit !($pct < $floor) }"; then
  echo "${GROUP}: ${pct}% < floor ${floor}%" >&2
  exit 1
fi

echo "${GROUP}: ${pct}% >= floor ${floor}%"
exit 0
