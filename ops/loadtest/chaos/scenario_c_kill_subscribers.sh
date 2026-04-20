#!/usr/bin/env bash
#
# Chaos C — kill 10 % of active SSE subscribers mid-publish.
#
# Strategy: identify the taskapi PID, list its established TCP
# connections to port 8080 where the peer is a loopback client,
# pick ~10 % at random, and RST them with `ss -K` (Linux) or
# `tcpdrop` (macOS). The script expects scenario_a_fanout.js to
# already be running in another terminal so there are >= 100 live
# subscribers.
#
# Pass criterion: every killed client reconnects within 5 s and
# replays missed events via Last-Event-ID without emitting an extra
# resync frame. Operator verifies by watching
# taskapi_sse_resync_emitted_total in Grafana — it should stay
# roughly flat through the kill window.
#
# Usage: bash scenario_c_kill_subscribers.sh [PORT]
#
# Requires root on Linux (ss -K is privileged). Exits non-zero if the
# platform is unsupported so CI skips gracefully.

set -euo pipefail

PORT="${1:-8080}"

if [[ "$(uname)" == "Linux" ]]; then
  if ! command -v ss >/dev/null; then
    echo "ss not installed; skipping"; exit 0
  fi
  # List ESTABLISHED connections to :$PORT with non-zero send-queue
  # (approx = active streaming) and pick 10 %. `ss -K` requires root.
  mapfile -t CONNS < <(ss -tnHo "sport = :$PORT" state established 2>/dev/null \
    | awk '{print $4" "$5}')
  TOTAL="${#CONNS[@]}"
  if (( TOTAL < 10 )); then
    echo "only $TOTAL established connections on :$PORT; start scenario_a_fanout.js first" >&2
    exit 1
  fi
  KILL_COUNT=$(( TOTAL / 10 ))
  echo "killing $KILL_COUNT of $TOTAL established connections on :$PORT"
  # Shuffle and take KILL_COUNT.
  printf '%s\n' "${CONNS[@]}" | shuf -n "$KILL_COUNT" | while read -r LOCAL REMOTE; do
    # ss -K accepts a filter; dst must match the remote peer we saw.
    sudo ss -K dst "${REMOTE%:*}" dport = "${REMOTE##*:}" >/dev/null || true
  done
elif [[ "$(uname)" == "Darwin" ]]; then
  if ! command -v tcpdrop >/dev/null; then
    echo "tcpdrop not installed (brew install tcpdrop); skipping"; exit 0
  fi
  # macOS: lsof lists peers; we RST them with tcpdrop.
  mapfile -t CONNS < <(sudo lsof -iTCP:"$PORT" -sTCP:ESTABLISHED -Pn 2>/dev/null \
    | awk 'NR>1{print $9}')
  TOTAL="${#CONNS[@]}"
  if (( TOTAL < 10 )); then
    echo "only $TOTAL established connections on :$PORT; start scenario_a_fanout.js first" >&2
    exit 1
  fi
  KILL_COUNT=$(( TOTAL / 10 ))
  echo "killing $KILL_COUNT of $TOTAL established connections on :$PORT"
  printf '%s\n' "${CONNS[@]}" | shuf -n "$KILL_COUNT" | while read -r CONN; do
    LOCAL="${CONN%%->*}"
    REMOTE="${CONN##*->}"
    sudo tcpdrop "${LOCAL%:*}" "${LOCAL##*:}" "${REMOTE%:*}" "${REMOTE##*:}" || true
  done
else
  echo "unsupported platform $(uname); scenario requires Linux or macOS" >&2
  exit 2
fi

echo "done — watch taskapi_sse_resync_emitted_total and rum_sse_reconnected_total for the next 60s"
