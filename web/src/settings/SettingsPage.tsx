import { type FormEvent, useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import type { AppSettings, AppSettingsPatch } from "@/api/settings";
import { listCursorModels } from "@/api/settings";
import {
  DEFAULT_APP_TIMEZONE,
  formatInAppTimezone,
  supportedTimezones,
} from "@/shared/time/appTimezone";
import { useAppSettings } from "./useAppSettings";
import "./settings.css";

const RUNNERS = [{ id: "cursor", label: "Cursor (cursor-agent CLI)" }] as const;

type FormState = {
  workerEnabled: boolean;
  agentPaused: boolean;
  runner: string;
  repoRoot: string;
  cursorBin: string;
  cursorModel: string;
  maxRunDurationSeconds: string;
  agentPickupDelaySeconds: string;
  displayTimezone: string;
  optimisticMutationsEnabled: boolean;
  sseReplayEnabled: boolean;
};

function toFormState(s: AppSettings): FormState {
  return {
    workerEnabled: s.worker_enabled,
    agentPaused: s.agent_paused,
    runner: s.runner,
    repoRoot: s.repo_root,
    cursorBin: s.cursor_bin,
    cursorModel: s.cursor_model,
    maxRunDurationSeconds: String(s.max_run_duration_seconds),
    agentPickupDelaySeconds: String(s.agent_pickup_delay_seconds),
    displayTimezone: s.display_timezone,
    optimisticMutationsEnabled: s.optimistic_mutations_enabled,
    sseReplayEnabled: s.sse_replay_enabled,
  };
}

function diffPatch(initial: AppSettings, form: FormState): AppSettingsPatch {
  const out: AppSettingsPatch = {};
  if (initial.worker_enabled !== form.workerEnabled) {
    out.worker_enabled = form.workerEnabled;
  }
  // agent_paused is intentionally NOT part of the form-driven diff:
  // the UI shows the flag as a read-only status badge (humans flip
  // pause through automation: agents, scripts, or another tool that
  // calls PATCH /settings directly). Including it here would round-trip
  // the user's last fetched value back as a no-op write and, worse,
  // would race a parallel agent flip — the SettingsPage would
  // silently undo a script's pause just because the operator hit
  // Save on an unrelated field.
  if (initial.runner !== form.runner.trim()) {
    out.runner = form.runner.trim();
  }
  if (initial.repo_root !== form.repoRoot.trim()) {
    out.repo_root = form.repoRoot.trim();
  }
  if (initial.cursor_bin !== form.cursorBin.trim()) {
    out.cursor_bin = form.cursorBin.trim();
  }
  if (initial.cursor_model !== form.cursorModel.trim()) {
    out.cursor_model = form.cursorModel.trim();
  }
  const parsedMax = Number.parseInt(form.maxRunDurationSeconds.trim() || "0", 10);
  if (Number.isFinite(parsedMax) && parsedMax !== initial.max_run_duration_seconds) {
    out.max_run_duration_seconds = parsedMax;
  }
  const parsedPickup = Number.parseInt(
    form.agentPickupDelaySeconds.trim() || "0",
    10,
  );
  if (
    Number.isFinite(parsedPickup) &&
    parsedPickup !== initial.agent_pickup_delay_seconds
  ) {
    out.agent_pickup_delay_seconds = parsedPickup;
  }
  const tzTrimmed = form.displayTimezone.trim();
  if (tzTrimmed !== "" && tzTrimmed !== initial.display_timezone) {
    out.display_timezone = tzTrimmed;
  }
  if (initial.optimistic_mutations_enabled !== form.optimisticMutationsEnabled) {
    out.optimistic_mutations_enabled = form.optimisticMutationsEnabled;
  }
  if (initial.sse_replay_enabled !== form.sseReplayEnabled) {
    out.sse_replay_enabled = form.sseReplayEnabled;
  }
  return out;
}

/**
 * Tagged feedback the page surfaces after a mutation settles. Split into
 * `success` vs `error` so each kind can render through the right ARIA
 * live-region (`role="status"` polite for success; `role="alert"`
 * assertive for errors). Previously a single `statusMsg: string` was
 * routed through `role="status"` for *both* kinds, which meant
 * screen-reader users with assistive tech configured for
 * polite-only announcements could miss failures, AND the visual
 * treatment didn't distinguish the two (a successful save and a
 * probe-failed-with-error rendered with the same neutral styling).
 *
 * `null` is the steady idle state; setting `status` always replaces
 * the previous one (no stacking — each mutation supersedes the prior
 * feedback).
 */
type Status = { kind: "success"; message: string } | { kind: "error"; message: string } | null;

export function SettingsPage() {
  useDocumentTitle("Settings");
  const { settings, isLoading, error, patch, probe, refetch } =
    useAppSettings();
  const [form, setForm] = useState<FormState | null>(null);
  const [status, setStatus] = useState<Status>(null);
  /**
   * The PATH-resolved cursor binary the server reports from the most
   * recent successful probe, kept around so the help text can show the
   * concrete default ("auto-detected at /usr/local/bin/cursor-agent")
   * after the operator clicks Test with the field blank. Cleared on
   * any field edit because a fresh edit may invalidate the previously
   * resolved path (e.g. they're typing a custom path that hasn't been
   * probed yet).
   */
  const [resolvedDefaultBin, setResolvedDefaultBin] = useState<string | null>(
    null,
  );

  useEffect(() => {
    if (settings && form === null) {
      setForm(toFormState(settings));
    }
  }, [settings, form]);

  const isDirty = useMemo(() => {
    if (!settings || !form) return false;
    return Object.keys(diffPatch(settings, form)).length > 0;
  }, [settings, form]);

  const cursorModelsQuery = useQuery({
    queryKey: [
      "settings",
      "cursor-models",
      settings?.cursor_bin,
      form?.cursorBin,
      form?.runner,
    ],
    queryFn: ({ signal }) =>
      listCursorModels(
        {
          runner: form?.runner ?? settings?.runner ?? "cursor",
          binary_path: (form?.cursorBin ?? "").trim() || undefined,
        },
        { signal },
      ),
    enabled: Boolean(settings && form),
  });

  const modelIdsFromList = useMemo(() => {
    const m = cursorModelsQuery.data;
    if (!m?.ok || !m.models) return new Set<string>();
    return new Set(m.models.map((x) => x.id));
  }, [cursorModelsQuery.data]);

  const maxParsed = form ? Number.parseInt(form.maxRunDurationSeconds.trim() || "0", 10) : 0;
  const maxInvalid = form ? !Number.isFinite(maxParsed) || maxParsed < 0 : false;
  const pickupParsed = form
    ? Number.parseInt(form.agentPickupDelaySeconds.trim() || "0", 10)
    : 0;
  const pickupInvalid = form
    ? !Number.isFinite(pickupParsed) ||
      pickupParsed < 0 ||
      pickupParsed > 604800
    : false;

  function handleField<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((cur) => (cur === null ? cur : { ...cur, [key]: value }));
    // The cursor-bin field's resolved default is only meaningful while
    // the field stays blank; once the operator starts typing, the
    // previous resolution describes a path they're no longer using.
    if (key === "cursorBin" && resolvedDefaultBin !== null) {
      setResolvedDefaultBin(null);
    }
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!settings || !form || maxInvalid || pickupInvalid) return;
    const body = diffPatch(settings, form);
    if (Object.keys(body).length === 0) return;
    // Snapshot the form *as submitted* so we can detect post-submit
    // typing on any field when the PATCH eventually resolves. The
    // race we're closing here: the user clicks Save with one field
    // edited (e.g. `repo_root`), then while the PATCH is in flight
    // they keep typing in *other* fields (e.g. `cursor_bin`).
    // Without this snapshot the post-resolution
    // `setForm(toFormState(next))` would clobber the in-flight
    // typing back to whatever the server returned (which for the
    // un-submitted fields is the *prior* server value, not the
    // user's typing) — silent data loss with no feedback.
    //
    // Same race-hardening shape used by `useTaskPatchFlow` /
    // `useTaskDeleteFlow` / `evaluateDraftMutation` — capture the
    // freshest known good snapshot at call time, then per-field
    // compare at resolve time and only apply server truth for
    // fields the user hasn't subsequently edited. Fields the user
    // re-edited keep the user's typing; `isDirty` will recompute
    // against the new server-known baseline so the Save button
    // re-enables for the new edits.
    const formAtSubmit = form;
    setStatus(null);
    try {
      const next = await patch.mutateAsync(body);
      setForm((cur) => {
        if (cur === null) return toFormState(next);
        const merged: FormState = { ...cur };
        if (cur.workerEnabled === formAtSubmit.workerEnabled) {
          merged.workerEnabled = next.worker_enabled;
        }
        if (cur.runner === formAtSubmit.runner) {
          merged.runner = next.runner;
        }
        if (cur.repoRoot === formAtSubmit.repoRoot) {
          merged.repoRoot = next.repo_root;
        }
        if (cur.cursorBin === formAtSubmit.cursorBin) {
          merged.cursorBin = next.cursor_bin;
        }
        if (cur.cursorModel === formAtSubmit.cursorModel) {
          merged.cursorModel = next.cursor_model;
        }
        if (cur.maxRunDurationSeconds === formAtSubmit.maxRunDurationSeconds) {
          merged.maxRunDurationSeconds = String(next.max_run_duration_seconds);
        }
        if (cur.agentPickupDelaySeconds === formAtSubmit.agentPickupDelaySeconds) {
          merged.agentPickupDelaySeconds = String(
            next.agent_pickup_delay_seconds,
          );
        }
        if (cur.displayTimezone === formAtSubmit.displayTimezone) {
          merged.displayTimezone = next.display_timezone;
        }
        if (
          cur.optimisticMutationsEnabled === formAtSubmit.optimisticMutationsEnabled
        ) {
          merged.optimisticMutationsEnabled = next.optimistic_mutations_enabled;
        }
        if (cur.sseReplayEnabled === formAtSubmit.sseReplayEnabled) {
          merged.sseReplayEnabled = next.sse_replay_enabled;
        }
        return merged;
      });
      setStatus({ kind: "success", message: "Settings saved." });
    } catch (err) {
      setStatus({
        kind: "error",
        message: err instanceof Error ? err.message : String(err),
      });
    }
  }

  async function handleProbe() {
    if (!form) return;
    setStatus(null);
    try {
      const result = await probe.mutateAsync({
        runner: form.runner.trim() || undefined,
        binary_path: form.cursorBin.trim() || undefined,
      });
      if (result.ok) {
        // Compose the message from whichever fields the server populated.
        // The resolved binary path is the most user-actionable bit when
        // the operator left the field blank — without it they have no
        // idea what "auto-detect on PATH" actually picked.
        const bits: string[] = ["Cursor binary OK"];
        if (result.binary_path) bits.push(`at ${result.binary_path}`);
        if (result.version) bits.push(`(version ${result.version})`);
        setStatus({ kind: "success", message: `${bits.join(" ")}.` });
        if (result.binary_path && form.cursorBin.trim() === "") {
          setResolvedDefaultBin(result.binary_path);
        } else {
          setResolvedDefaultBin(null);
        }
      } else {
        // Probe returned `{ ok: false, error }` — semantically a
        // failure even though the HTTP request succeeded. Route through
        // the error channel so screen readers hear the assertive
        // announcement and the user sees the danger styling instead of
        // the previous "neutral status" treatment that made a probe
        // failure look like a successful informational message.
        setStatus({
          kind: "error",
          message: `Cursor binary check failed: ${result.error ?? "unknown error"}`,
        });
      }
    } catch (err) {
      setStatus({
        kind: "error",
        message: err instanceof Error ? err.message : String(err),
      });
    }
  }

  if (isLoading || !form || !settings) {
    return (
      <section className="settings-page" aria-busy="true">
        <h2 className="settings-page-title term-arrow">
          <span>Settings</span>
        </h2>
        <p>{error ? `Error: ${error.message}` : "Loading settings…"}</p>
        {error ? (
          <button type="button" onClick={() => void refetch()}>
            Retry
          </button>
        ) : null}
      </section>
    );
  }

  const repoRootEmpty = form.repoRoot.trim() === "";
  const lastUpdated = settings.updated_at ?? "";
  const tzOptions = supportedTimezones();
  // Tolerate operator-pasted custom zones that aren't in the
  // Intl.supportedValuesOf list — show them as a synthetic option so
  // the <select> can display the saved value rather than silently
  // falling back to the first list item.
  const showCustomTz =
    form.displayTimezone.trim() !== "" &&
    !tzOptions.includes(form.displayTimezone.trim());
  const lastUpdatedFormatted = lastUpdated
    ? formatInAppTimezone(lastUpdated, form.displayTimezone || DEFAULT_APP_TIMEZONE)
    : "";

  return (
    <section className="settings-page">
      <header className="settings-page-header">
        <h2 className="settings-page-title term-arrow">
          <span>Settings</span>
        </h2>
        {lastUpdated ? (
          <p className="settings-page-subtitle" data-testid="settings-last-updated">
            Last updated:{" "}
            <time dateTime={lastUpdated}>{lastUpdatedFormatted || lastUpdated}</time>
          </p>
        ) : null}
      </header>

      {repoRootEmpty ? (
        <div role="alert" className="settings-banner settings-banner--warn">
          <strong>Workspace not configured.</strong> Set the repository root
          below to enable the agent worker, file mentions, and the
          <code> /repo/* </code>endpoints.
        </div>
      ) : null}

      <form className="settings-form" onSubmit={(e) => void handleSubmit(e)}>
        <fieldset className="settings-fieldset">
          <legend>Agent worker</legend>
          <label className="settings-field settings-field--inline">
            <input
              type="checkbox"
              checked={form.workerEnabled}
              onChange={(e) => handleField("workerEnabled", e.target.checked)}
            />
            <span className="settings-field-label">Enable agent worker</span>
          </label>
          <p className="settings-field-help">
            When enabled, the worker pulls ready tasks and dispatches them to
            the configured runner.
          </p>

          <div
            className="settings-field settings-field--inline"
            data-testid="settings-agent-paused-status"
            data-paused={form.agentPaused ? "true" : "false"}
          >
            <span className="settings-field-label">Agent pause status</span>
            <span
              className={`settings-pill ${
                form.agentPaused
                  ? "settings-pill--paused"
                  : "settings-pill--running"
              }`}
              role="status"
            >
              {form.agentPaused ? "Paused" : "Running"}
            </span>
          </div>
          <p className="settings-field-help">
            Read-only. The pause flag is operated by automation (agents
            and scripts that <code>PATCH /settings</code> with{" "}
            <code>{`{"agent_paused": true}`}</code>) so a human cannot
            silently undo a script's pause by saving an unrelated field.
            Use <em>Enable agent worker</em> above for a long-term
            shutdown. The header chip turns amber while paused.
          </p>

          <label className="settings-field">
            <span className="settings-field-label">Runner</span>
            <select
              value={form.runner}
              onChange={(e) => handleField("runner", e.target.value)}
            >
              {RUNNERS.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.label}
                </option>
              ))}
              {RUNNERS.find((r) => r.id === form.runner) ? null : (
                <option value={form.runner}>{form.runner} (custom)</option>
              )}
            </select>
          </label>

          <label className="settings-field">
            <span className="settings-field-label">
              Agent pickup delay (seconds)
            </span>
            <input
              type="number"
              min={0}
              max={604800}
              step={1}
              placeholder="5"
              value={form.agentPickupDelaySeconds}
              onChange={(e) =>
                handleField("agentPickupDelaySeconds", e.target.value)
              }
              aria-invalid={pickupInvalid}
            />
          </label>
          <p className="settings-field-help">
            Minimum wait before the worker runs a new ready task. Default{" "}
            <code>5</code>s. <code>0</code> = no wait.
          </p>
          {pickupInvalid ? (
            <p role="alert" className="settings-field-error">
              Must be between 0 and 604800 (7 days).
            </p>
          ) : null}
        </fieldset>

        <fieldset className="settings-fieldset">
          <legend>Display</legend>
          <label className="settings-field">
            <span className="settings-field-label">Timezone</span>
            <select
              data-testid="settings-display-timezone-select"
              value={form.displayTimezone}
              onChange={(e) => handleField("displayTimezone", e.target.value)}
            >
              {tzOptions.map((tz) => (
                <option key={tz} value={tz}>
                  {tz}
                </option>
              ))}
              {showCustomTz ? (
                <option value={form.displayTimezone.trim()}>
                  {form.displayTimezone.trim()} (saved — not in current list)
                </option>
              ) : null}
            </select>
          </label>
          <p className="settings-field-help">
            Operator-facing timestamps (scheduled task pickup time,
            &quot;last updated&quot;, etc.) render in this timezone. The wire
            format every API uses stays RFC3339 UTC; this only affects
            display. Default <code>UTC</code>.
          </p>
        </fieldset>

        <fieldset className="settings-fieldset">
          <legend>Realtime rollout</legend>
          <label className="settings-field settings-field--inline">
            <input
              type="checkbox"
              data-testid="settings-optimistic-mutations-toggle"
              checked={form.optimisticMutationsEnabled}
              onChange={(e) =>
                handleField("optimisticMutationsEnabled", e.target.checked)
              }
            />
            <span className="settings-field-label">
              Optimistic mutations enabled
            </span>
          </label>
          <p className="settings-field-help">
            When on, the SPA renders mutation results (status change,
            delete, checklist, requeue, subtask create) immediately and
            rolls back on server error. When off, every mutation waits
            for the server round-trip before the UI updates (the legacy
            pessimistic path). Leave off until a full SLO window of
            green rollback-rate and error-rate metrics in staging. See{" "}
            <code>docs/SLOs.md</code>.
          </p>
          <label className="settings-field settings-field--inline">
            <input
              type="checkbox"
              data-testid="settings-sse-replay-toggle"
              checked={form.sseReplayEnabled}
              onChange={(e) => handleField("sseReplayEnabled", e.target.checked)}
            />
            <span className="settings-field-label">
              SSE replay enabled (lossless events)
            </span>
          </label>
          <p className="settings-field-help">
            When on, the <code>/events</code> stream honors the
            browser&apos;s <code>Last-Event-ID</code> header on reconnect
            and replays buffered events so the SPA doesn&apos;t miss
            mutations during a brief disconnect. Off = live-only fanout
            (reconnect = cold start). Purely additive server-side; the
            SPA&apos;s resume header is a no-op when this flag is off.
          </p>
        </fieldset>

        <fieldset className="settings-fieldset">
          <legend>Workspace</legend>
          <label className="settings-field">
            <span className="settings-field-label">Repository root (absolute path)</span>
            <input
              type="text"
              value={form.repoRoot}
              onChange={(e) => handleField("repoRoot", e.target.value)}
              placeholder="/Users/me/code/my-project"
              spellCheck={false}
              autoComplete="off"
            />
          </label>
          <p className="settings-field-help">
            The project the agent will execute tasks on. The agent worker,{" "}
            <code>/repo/*</code> endpoints, and <code>@file</code> mentions
            all read this path. Leave empty to disable repo features until
            you pick a workspace.
          </p>
        </fieldset>

        <fieldset className="settings-fieldset" id="cursor-agent">
          <legend>Cursor agent (CLI)</legend>
          <label className="settings-field">
            <span className="settings-field-label">Model override</span>
            <select
              data-testid="settings-cursor-model-select"
              value={form.cursorModel}
              onChange={(e) => handleField("cursorModel", e.target.value)}
              disabled={cursorModelsQuery.isFetching}
              aria-busy={cursorModelsQuery.isFetching}
            >
              <option value="">
                Default (omit --model; Cursor chooses for your account)
              </option>
              {cursorModelsQuery.data?.ok && cursorModelsQuery.data.models
                ? cursorModelsQuery.data.models.map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.label}
                    </option>
                  ))
                : null}
              {form.cursorModel.trim() !== "" &&
              !modelIdsFromList.has(form.cursorModel.trim()) ? (
                <option value={form.cursorModel.trim()}>
                  {form.cursorModel.trim()} (saved — not in current list)
                </option>
              ) : null}
            </select>
          </label>
          {cursorModelsQuery.isError ? (
            <p role="alert" className="settings-field-error">
              Could not load models from the Cursor CLI:{" "}
              {cursorModelsQuery.error instanceof Error
                ? cursorModelsQuery.error.message
                : String(cursorModelsQuery.error)}
            </p>
          ) : null}
          {cursorModelsQuery.data && !cursorModelsQuery.data.ok ? (
            <p role="alert" className="settings-field-error">
              {cursorModelsQuery.data.error ?? "Model list failed."}
            </p>
          ) : null}
          <p className="settings-field-help">
            List comes from <code>cursor-agent --list-models</code> using the
            binary path above. Pick a model here or leave &quot;Default&quot; to
            omit <code>--model</code>. After a usage-limit error, choose another
            model and save.
          </p>

          <label className="settings-field">
            <span className="settings-field-label">Cursor CLI path</span>
            <input
              type="text"
              value={form.cursorBin}
              onChange={(e) => handleField("cursorBin", e.target.value)}
              placeholder="/usr/local/bin/cursor-agent"
              spellCheck={false}
              autoComplete="off"
            />
          </label>
          <p className="settings-field-help">
            Leave empty to auto-detect on PATH. Use the test button to verify
            before saving.
          </p>
          {form.cursorBin.trim() === "" && resolvedDefaultBin ? (
            <div className="settings-resolved-bin">
              <span className="settings-resolved-bin-label">
                Currently resolves to
              </span>
              <code
                className="settings-resolved-bin-path"
                data-testid="settings-resolved-cursor-bin"
              >
                {resolvedDefaultBin}
              </code>
            </div>
          ) : null}
          <button
            type="button"
            className="secondary"
            onClick={() => void handleProbe()}
            disabled={probe.isPending}
          >
            {probe.isPending ? "Testing…" : "Test cursor binary"}
          </button>
        </fieldset>

        <fieldset className="settings-fieldset">
          <legend>Run timeout</legend>
          <label className="settings-field">
            <span className="settings-field-label">Max run duration (seconds)</span>
            <input
              type="number"
              min={0}
              step={1}
              value={form.maxRunDurationSeconds}
              onChange={(e) => handleField("maxRunDurationSeconds", e.target.value)}
              aria-invalid={maxInvalid}
            />
          </label>
          <p className="settings-field-help">
            Maximum wall-clock time per agent run. Set to <code>0</code> for
            no limit.
          </p>
          {maxInvalid ? (
            <p role="alert" className="settings-field-error">
              Must be a non-negative integer.
            </p>
          ) : null}
        </fieldset>

        <div className="settings-actions">
          <button
            type="submit"
            disabled={!isDirty || patch.isPending || maxInvalid || pickupInvalid}
          >
            {patch.isPending ? "Saving…" : "Save changes"}
          </button>
        </div>

        {status?.kind === "success" ? (
          <p
            role="status"
            data-testid="settings-status"
            className="settings-status"
          >
            {status.message}
          </p>
        ) : null}
        {status?.kind === "error" ? (
          // Errors deliberately render through `MutationErrorBanner`
          // (role="alert", aria-live="assertive" implicitly) so
          // screen-readers announce them immediately, and so the
          // visual treatment (`.err` danger background) makes the
          // failure unmistakable. The prior single-channel
          // `role="status"` rendering was an a11y regression for
          // anyone with assistive tech configured for polite-only
          // announcements (a missed save / probe failure could go
          // entirely unnoticed). The
          // `data-testid="settings-status-error"` selector lets the
          // existing test plumbing assert against this region
          // explicitly without depending on the message phrase.
          <div data-testid="settings-status-error">
            <MutationErrorBanner
              error={status.message}
              className="settings-status-err"
            />
          </div>
        ) : null}
      </form>
    </section>
  );
}
