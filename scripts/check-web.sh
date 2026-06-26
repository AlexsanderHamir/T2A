#!/usr/bin/env bash
# Hamix web verification — source of truth for the CI web job.
#
# Steps: npm ci (--install), web test, web lint, web standards, web build
#
# Usage (repo root): ./scripts/check-web.sh [flags]
#
# Flags:
#   --verbose, -v       Stream full tool output (CI uses this)
#   --install           Run npm ci in web/ before other steps
#   --group=<name>      Restrict to lint|build|test-unit|test-components|test-app|test-task-pages|test-task-create|test-settings|test-projects|test-worktrees (CI matrix)
#   --help, -h          Show options
#
# CI:
#   ./scripts/check-web.sh --install --verbose --group=lint
#   ./scripts/check-web.sh --install --verbose --group=build
#   ./scripts/check-web.sh --install --verbose --group=test-unit
#   ./scripts/check-web.sh --install --verbose --group=test-components
#   ./scripts/check-web.sh --install --verbose --group=test-app
#   ./scripts/check-web.sh --install --verbose --group=test-task-pages
#   ./scripts/check-web.sh --install --verbose --group=test-task-create
#   ./scripts/check-web.sh --install --verbose --group=test-settings
#   ./scripts/check-web.sh --install --verbose --group=test-projects
#   ./scripts/check-web.sh --install --verbose --group=test-worktrees

set -uo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"

VERBOSE=0
INSTALL=0
GROUP=""

show_help() {
  sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --verbose|-v) VERBOSE=1; shift ;;
    --install) INSTALL=1; shift ;;
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

if [[ -n "$GROUP" ]]; then
  case "$GROUP" in
    lint|build|test-unit|test-components|test-app|test-task-pages|test-task-create|test-settings|test-projects|test-worktrees) ;;
    *)
      echo "unknown web group: $GROUP (valid: lint build test-unit test-components test-app test-task-pages test-task-create test-settings test-projects test-worktrees)" >&2
      exit 2
      ;;
  esac
fi

if [[ ! -f web/package.json ]]; then
  echo "web/package.json not found" >&2
  exit 1
fi

CHECK_BANNER="Hamix check (web)"
CHECK_SECTION="web"
CHECK_START=$SECONDS
STEP=0
PASSED=0

if [[ -n "$GROUP" ]]; then
  case "$GROUP" in
    lint) TOTAL=3 ;;
    build) TOTAL=1 ;;
    test-unit|test-components|test-app|test-task-pages|test-task-create|test-settings|test-projects|test-worktrees) TOTAL=1 ;;
  esac
  [[ "$INSTALL" -eq 1 ]] && TOTAL=$((TOTAL + 1))
else
  TOTAL=12
  [[ "$INSTALL" -eq 1 ]] && TOTAL=13
fi

# shellcheck source=check-lib.sh
source "$(dirname "$0")/check-lib.sh"

web_test_stats() {
  local log="$1"
  if grep -qE 'Tests +[0-9]+ passed' "$log" 2>/dev/null; then
    STEP_STATS="$(grep -oE 'Tests +[0-9]+ passed' "$log" | tail -1 | sed 's/Tests /tests /')"
  fi
}

web_lint_stats() {
  local log="$1"
  local warnings
  warnings="$(grep -oE '[0-9]+ warnings' "$log" 2>/dev/null | tail -1 || true)"
  if [[ -n "$warnings" && "$warnings" != "0 warnings" ]]; then
    STEP_STATS="$warnings"
  fi
}

run_web_test() {
  local label="$1"
  shift
  local start=$SECONDS
  local log
  log="$(mktemp "${TMPDIR:-/tmp}/hamix-check.XXXXXX")"
  local reporter_args=(--run "$@")
  if [[ "$VERBOSE" != "1" ]]; then
    reporter_args+=(--reporter=basic)
  fi

  step_prefix
  printf '%s ' "$label"

  if [[ "$VERBOSE" == "1" ]]; then
    echo "${C_CYAN}...${C_RESET}"
    set +e
    npm test -- "${reporter_args[@]}"
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
  npm test -- "${reporter_args[@]}" >"$log" 2>&1
  local code=$?
  set -e
  local elapsed=$((SECONDS - start))
  add_section_time "$elapsed"

  if [[ $code -eq 0 ]]; then
    web_test_stats "$log"
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

run_web_lint() {
  local label="web (lint)"
  local start=$SECONDS
  local log
  log="$(mktemp "${TMPDIR:-/tmp}/hamix-check.XXXXXX")"

  step_prefix
  printf '%s ' "$label"

  if [[ "$VERBOSE" == "1" ]]; then
    echo "${C_CYAN}...${C_RESET}"
    set +e
    npm run lint
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
  npm run lint >"$log" 2>&1
  local code=$?
  set -e
  local elapsed=$((SECONDS - start))
  add_section_time "$elapsed"

  if [[ $code -eq 0 ]]; then
    web_lint_stats "$log"
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

maybe_npm_ci() {
  if [[ "$INSTALL" -eq 1 ]]; then
    run_cmd "npm ci" bash -c 'cd web && npm ci'
  fi
}

print_banner

case "$GROUP" in
  lint)
    run_cmd "check-brand" bash "$(dirname "$0")/check-brand.sh"
    maybe_npm_ci
    pushd web >/dev/null
    run_web_lint
    run_cmd "web standards" npm run check:standards
    popd >/dev/null
    complete_ok
    ;;
  build)
    maybe_npm_ci
    pushd web >/dev/null
    run_cmd "web (build)" npm run build
    popd >/dev/null
    complete_ok
    ;;
  test-unit)
    maybe_npm_ci
    pushd web >/dev/null
    run_web_test "web (test-unit)" --project=unit
    popd >/dev/null
    complete_ok
    ;;
  test-components)
    maybe_npm_ci
    pushd web >/dev/null
    run_web_test "web (test-components)" --project=components
    popd >/dev/null
    complete_ok
    ;;
  test-app)
    maybe_npm_ci
    pushd web >/dev/null
    run_web_test "web (test-app)" --project=app
    popd >/dev/null
    complete_ok
    ;;
  test-task-pages)
    maybe_npm_ci
    pushd web >/dev/null
    run_web_test "web (test-task-pages)" --project=task-pages
    popd >/dev/null
    complete_ok
    ;;
  test-task-create)
    maybe_npm_ci
    pushd web >/dev/null
    run_web_test "web (test-task-create)" --project=task-create
    popd >/dev/null
    complete_ok
    ;;
  test-settings)
    maybe_npm_ci
    pushd web >/dev/null
    run_web_test "web (test-settings)" --project=settings
    popd >/dev/null
    complete_ok
    ;;
  test-projects)
    maybe_npm_ci
    pushd web >/dev/null
    run_web_test "web (test-projects)" --project=projects
    popd >/dev/null
    complete_ok
    ;;
  test-worktrees)
    maybe_npm_ci
    pushd web >/dev/null
    run_web_test "web (test-worktrees)" --project=worktrees
    popd >/dev/null
    complete_ok
    ;;
esac

# Full local bar (no --group)
run_cmd "check-brand" bash "$(dirname "$0")/check-brand.sh"
maybe_npm_ci

pushd web >/dev/null
run_web_test "web (test-unit)" --project=unit
run_web_test "web (test-components)" --project=components
run_web_test "web (test-app)" --project=app
run_web_test "web (test-task-pages)" --project=task-pages
run_web_test "web (test-task-create)" --project=task-create
run_web_test "web (test-settings)" --project=settings
run_web_test "web (test-projects)" --project=projects
run_web_test "web (test-worktrees)" --project=worktrees
run_web_lint
run_cmd "web standards" npm run check:standards
run_cmd "web (build)" npm run build
popd >/dev/null

complete_ok
