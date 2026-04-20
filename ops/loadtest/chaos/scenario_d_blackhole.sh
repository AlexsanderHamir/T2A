#!/usr/bin/env bash
#
# Chaos D — 30 s network blackhole on the SSE port.
#
# Installs a firewall rule that silently drops inbound traffic to
# port 8080 for 30 s, then restores. Subscribers see their
# EventSource timeout after the server-side IdleTimeout; on the
# other side of the blackhole they reconnect and replay via
# Last-Event-ID.
#
# Pass criterion: zero data loss (no resync escalation beyond the
# ring-buffer-overflow threshold) and UI state in the Playwright
# scenario remains consistent with server truth after the blackhole
# lifts.
#
# Usage: sudo bash scenario_d_blackhole.sh [PORT] [SECONDS]
#
# Requires root (iptables on Linux, pfctl on macOS).

set -euo pipefail

PORT="${1:-8080}"
SECONDS_DOWN="${2:-30}"

cleanup() {
  if [[ "$(uname)" == "Linux" ]]; then
    sudo iptables -D INPUT -p tcp --dport "$PORT" -j DROP 2>/dev/null || true
  elif [[ "$(uname)" == "Darwin" ]]; then
    sudo pfctl -F rules 2>/dev/null || true
    sudo pfctl -d 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

if [[ "$(uname)" == "Linux" ]]; then
  echo "blackhole: dropping inbound TCP :$PORT for ${SECONDS_DOWN}s"
  sudo iptables -A INPUT -p tcp --dport "$PORT" -j DROP
elif [[ "$(uname)" == "Darwin" ]]; then
  echo "blackhole: dropping inbound TCP :$PORT for ${SECONDS_DOWN}s (pf)"
  echo "block in quick proto tcp from any to any port $PORT" | sudo pfctl -e -f -
else
  echo "unsupported platform $(uname)" >&2
  exit 2
fi

sleep "$SECONDS_DOWN"

echo "blackhole: restored. Watch rum_sse_reconnected_total + taskapi_sse_resync_emitted_total for the next 60s"
