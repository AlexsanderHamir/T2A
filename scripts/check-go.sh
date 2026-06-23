#!/usr/bin/env bash
# Hamix Go verification — source of truth for the CI backend job.
#
# Steps: gofmt, go vet, scheduling boundary, go test, funclogmeasure
#
# Usage (repo root): ./scripts/check-go.sh [flags]
#
# Flags:
#   --verbose, -v       Stream full tool output (CI uses this)
#   --skip-funclog        Skip funclogmeasure -enforce
#   --lint-only           Lint steps only (includes test-group coverage guard)
#   --tests-only          go test only (use with --group for CI matrix cells)
#   --group=<name>        Restrict go test to core|tasks|agents|harness
#   --help, -h            Show options
#
# CI: ./scripts/check-go.sh --lint-only --verbose
#     ./scripts/check-go.sh --tests-only --group=core --verbose

set -uo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"

script_dir="$(dirname "$0")"
# shellcheck source=test-groups.sh
source "$script_dir/test-groups.sh"

VERBOSE=0
SKIP_FUNCLOG=0
LINT_ONLY=0
TESTS_ONLY=0
GROUP=""

show_help() {
  sed -n '2,18p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --verbose|-v) VERBOSE=1; shift ;;
    --skip-funclog) SKIP_FUNCLOG=1; shift ;;
    --lint-only) LINT_ONLY=1; shift ;;
    --tests-only) TESTS_ONLY=1; shift ;;
    --group=*) GROUP="${1#--group=}"; shift ;;
    --group)
      GROUP="${2:-}"
      shift 2
      ;;
    --help|-h) show_help; exit 0 ;;
    *)
      echo "unknown flag: $1 (try --help)" >&2
      exit 2
      ;;
  esac
done

if [[ "$LINT_ONLY" -eq 1 && "$TESTS_ONLY" -eq 1 ]]; then
  echo "cannot use --lint-only and --tests-only together" >&2
  exit 2
fi

if [[ -n "$GROUP" ]]; then
  if ! group_packages "$GROUP" >/dev/null 2>&1; then
    exit 2
  fi
fi

CHECK_BANNER="Hamix check (Go)"
CHECK_SECTION="go"
CHECK_START=$SECONDS
STEP=0
PASSED=0

if [[ "$TESTS_ONLY" -eq 1 ]]; then
  TOTAL=1
elif [[ "$LINT_ONLY" -eq 1 ]]; then
  if [[ "$SKIP_FUNCLOG" -eq 0 ]]; then
    TOTAL=6
  else
    TOTAL=5
  fi
else
  if [[ "$SKIP_FUNCLOG" -eq 0 ]]; then
    TOTAL=6
  else
    TOTAL=5
  fi
fi

# shellcheck source=check-lib.sh
source "$script_dir/check-lib.sh"

go_test_stats() {
  local log="$1"
  local count
  count="$(grep -cE '^(ok|FAIL|\?)' "$log" 2>/dev/null || true)"
  if [[ "$count" -gt 0 ]]; then
    STEP_STATS="${count} packages"
  fi
}

go_test_targets() {
  if [[ -n "$GROUP" ]]; then
    group_packages "$GROUP"
  else
    echo "./..."
  fi
}

step_gofmt() {
  local label="gofmt"
  local start=$SECONDS
  step_prefix
  printf '%s ' "$label"

  local unformatted=""
  while IFS= read -r -d '' file; do
    local line
    line="$(gofmt -l "$file")"
    if [[ -n "$line" ]]; then
      unformatted+="${line}"$'\n'
    fi
  done < <(find . -name '*.go' -not -path './vendor/*' -print0)

  local elapsed=$((SECONDS - start))
  add_section_time "$elapsed"

  if [[ -n "$unformatted" ]]; then
    echo "${C_RED}FAILED${C_RESET}"
    printf '%s' "$unformatted"
    fail_step "$label" 1 "gofmt -w on the files above"
  fi

  PASSED=$((PASSED + 1))
  print_ok_line "$label" "$elapsed"
}

step_scheduling_boundary() {
  local label="scheduling boundary"
  local start=$SECONDS
  step_prefix
  printf '%s ' "$label"

  local hits=""
  if rg -q 'gorm|store/|handler/|agents/' pkgs/tasks/scheduling/ -g '*.go' -g '!*_test.go' 2>/dev/null; then
    hits="$(rg -n 'gorm|store/|handler/|agents/' pkgs/tasks/scheduling/ -g '*.go' -g '!*_test.go' 2>/dev/null || true)"
  fi
  local elapsed=$((SECONDS - start))
  add_section_time "$elapsed"

  if [[ -n "$hits" ]]; then
    echo "${C_RED}FAILED${C_RESET}"
    echo "scheduling must not import persistence or transport:"
    echo "$hits"
    fail_step "$label" 1
  fi

  PASSED=$((PASSED + 1))
  print_ok_line "$label" "$elapsed"
}

step_test_group_coverage() {
  local label="test group coverage"
  local start=$SECONDS
  step_prefix
  printf '%s ' "$label"

  set +e
  assert_groups_cover_all
  local code=$?
  set -e

  local elapsed=$((SECONDS - start))
  add_section_time "$elapsed"

  if [[ $code -ne 0 ]]; then
    echo "${C_RED}FAILED${C_RESET}"
    fail_step "$label" "$code"
  fi

  PASSED=$((PASSED + 1))
  print_ok_line "$label" "$elapsed"
}

run_go_test() {
  local label="go test"
  if [[ -n "$GROUP" ]]; then
    label="go test ($GROUP)"
  fi
  local start=$SECONDS
  local log
  log="$(mktemp "${TMPDIR:-/tmp}/hamix-check.XXXXXX")"

  local targets
  targets="$(go_test_targets)"

  step_prefix
  printf '%s ' "$label"

  if [[ "$VERBOSE" == "1" ]]; then
    echo "${C_CYAN}...${C_RESET}"
    set +e
    # shellcheck disable=SC2086
    go test $targets -count=1
    local code=$?
    set -e
    local elapsed=$((SECONDS - start))
    add_section_time "$elapsed"
    if [[ $code -eq 0 ]]; then
      PASSED=$((PASSED + 1))
      return 0
    fi
    fail_step "$label" "$code"
  fi

  set +e
  # shellcheck disable=SC2086
  go test $targets -count=1 >"$log" 2>&1
  local code=$?
  set -e
  local elapsed=$((SECONDS - start))
  add_section_time "$elapsed"

  if [[ $code -eq 0 ]]; then
    go_test_stats "$log"
    PASSED=$((PASSED + 1))
    print_ok_line "$label" "$elapsed" "${STEP_STATS:-}"
    STEP_STATS=""
    rm -f "$log"
    return 0
  fi

  echo "${C_RED}FAILED${C_RESET}"
  cat "$log"
  rm -f "$log"
  fail_step "$label" "$code"
}

print_banner

if [[ "$TESTS_ONLY" -eq 1 ]]; then
  run_go_test
  complete_ok
fi

run_cmd "check-brand" bash "$script_dir/check-brand.sh"
step_gofmt
run_cmd "go vet" go vet ./...
step_scheduling_boundary

if [[ "$LINT_ONLY" -eq 1 ]]; then
  step_test_group_coverage
else
  run_go_test
fi

if [[ "$SKIP_FUNCLOG" -eq 0 ]]; then
  run_cmd "funclogmeasure" go run ./cmd/funclogmeasure -enforce
fi

if [[ "$LINT_ONLY" -eq 1 ]]; then
  complete_ok
fi

complete_ok
