#!/usr/bin/env bash
# Fail when store models or postgres.Migrate changes without a SchemaRevision bump.
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"

base="${CHECK_SCHEMA_REV_BASE:-}"
if [[ -z "$base" ]]; then
  if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    mapfile -t diff_files < <((git diff --name-only HEAD 2>/dev/null; git diff --name-only --cached HEAD 2>/dev/null; git ls-files --others --exclude-standard) | sort -u)
  else
    exit 0
  fi
else
  mapfile -t diff_files < <(git diff --name-only "$base"...HEAD 2>/dev/null || true)
fi

needs_bump=0
revision_touched=0
for f in "${diff_files[@]:-}"; do
  [[ -z "$f" ]] && continue
  case "$f" in
    pkgs/tasks/postgres/schema_revision.go) revision_touched=1 ;;
    pkgs/tasks/domain/*) needs_bump=1 ;;
    pkgs/tasks/store/model/*) needs_bump=1 ;;
    pkgs/tasks/postgres/postgres.go) needs_bump=1 ;;
  esac
done

if [[ "$needs_bump" -eq 0 ]]; then
  exit 0
fi
if [[ "$revision_touched" -eq 1 ]]; then
  exit 0
fi

echo "schema revision: changes under pkgs/tasks/domain, pkgs/tasks/store/model, or pkgs/tasks/postgres/postgres.go require bumping SchemaRevision in pkgs/tasks/postgres/schema_revision.go" >&2
exit 1
