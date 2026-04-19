# Runbook: TaskAPISSEDroppedFramesElevated

## What it means

The Prometheus counter **`taskapi_sse_dropped_frames_total`** advanced at a sustained rate above **0.5 frames/s** for **5 minutes**. Each increment represents one fanout frame that **`pkgs/tasks/handler/sse.go`** had to drop on the `default` branch of the `select` because a subscriber's bounded channel (capacity **32**) was full at the time of `Publish`.

**Why this matters:** drops are silent at the wire — the affected client simply stops receiving the missed events. SPA caches that depend on `task_updated:{id}` or `task_cycle_changed:{id}/{cycle_id}` will show stale data until the next manual refresh. The publisher is never blocked, so the rest of the system is unaffected, but the user staring at the slow connection sees a broken experience without any client-side error.

## Severity and escalation

- **Default:** `severity: warning`.
- **Escalate** to `critical` and page on-call if the drop rate stays above **5 frames/s** for **15 minutes** *or* coincides with a spike in **`taskapi_sse_subscribers`** (a stuck client may be holding many fanout slots open).

## Dashboards (Prometheus / Grafana)

1. **Drop rate (this alert):**

   ```promql
   rate(taskapi_sse_dropped_frames_total[5m])
   ```

2. **Subscriber count** (correlate with the drop rate to spot per-subscriber pressure):

   ```promql
   taskapi_sse_subscribers
   ```

3. **Drops per subscriber per second** (rough average; a value approaching 1.0 suggests every subscriber is dropping every fanout):

   ```promql
   rate(taskapi_sse_dropped_frames_total[5m]) / clamp_min(taskapi_sse_subscribers, 1)
   ```

4. **Concurrent in-flight requests** (a stuck SSE handler also bumps this — every connected SSE client holds an in-flight slot for as long as the connection stays open):

   ```promql
   taskapi_http_in_flight
   ```

## Logs (JSONL)

Each fanout drop emits a **Warn** line with **`operation`**=`tasks.sse.publish` and **`message`**=`sse fanout dropped frames`. The `subscribers` and `dropped` fields tell you the fanout shape at the time of the drop — `dropped` close to `subscribers` means every client was wedged; `dropped < subscribers` means only some clients are slow.

```bash
rg '"sse fanout dropped frames"' /var/log/taskapi/*.log
```

The drop log is emitted from the publisher goroutine (not from the slow subscriber's request context) so it does not carry a `request_id`. To find the candidate stuck client(s), look at all in-flight **`GET /events`** access lines with very large **`duration_ms`** — the one(s) that have been open the longest are usually the culprits:

```bash
rg '"route":"GET /events"' /var/log/taskapi/*.log | rg '"duration_ms":[0-9]{6,}'
```

(`duration_ms >= 100000` ≈ a connection that's been open for 100s+; SSE clients typically reconnect on the order of seconds when healthy.)

## Check first

1. **Subscriber count** (`taskapi_sse_subscribers`) — has it stepped up around the alert firing? A burst of new clients can briefly overwhelm fanout if any of them are slow to drain their initial backlog.
2. **`http.access` lines** for `GET /events` with very large `duration_ms` — these are the candidate stuck clients. The drop log itself is emitted from the publisher goroutine and does not carry the subscriber's `request_id`, so you cannot correlate 1:1; instead enumerate the longest-running `GET /events` connections and treat them as suspects.
3. **In-flight requests** (`taskapi_http_in_flight`) — sustained high values plus elevated drop rate point at SSE saturation, not application errors.
4. **Edge / proxy** — verify the reverse proxy isn't buffering responses (`X-Accel-Buffering: no` on `nginx`, `proxy-read-timeout` long enough for `KeepAliveSeconds`).

## Mitigations

- **Disconnect the offending client** if you can identify the `request_id`: sending a `SIGTERM` is too aggressive in production, but rolling the client (refresh the SPA tab, restart the operator script) usually frees the wedged channel.
- **Bump the fanout buffer** in `pkgs/tasks/handler/sse.go` (`Subscribe` allocates `make(chan string, 32)` — current cap **32**) **only after** confirming the slow client isn't a bug in the SPA. A larger buffer just trades drops for memory; it does not fix a stuck reader.
- **Reduce fanout volume** by collapsing chatty event sources (e.g. coalesce burst `task_updated` from the agent worker if it's spamming). Look at the `event_type` distribution in the `sse fanout dropped frames` logs — if one type dominates the drops, that's where to coalesce.
- **SSE-dedicated replicas** for high-fanout deployments: route `GET /events` to its own pool so a stuck SSE client cannot starve mutating-route capacity. Same idea as the `TaskAPIHTTPInFlightHigh` runbook, but specifically motivated by SSE.

## Related

- [`alert-in-flight-high.md`](./alert-in-flight-high.md) — covers the case where SSE saturation also drives total in-flight count above the alert threshold.
- [`docs/API-SSE.md`](../API-SSE.md) — wire-format reference for the SSE event types listed in the `sse fanout dropped frames` logs.
- [`docs/OBSERVABILITY.md`](../OBSERVABILITY.md) — full metric catalog including the original Session 24 PromQL example for this counter.
