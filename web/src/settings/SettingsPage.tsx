import { type FormEvent, useEffect, useMemo, useState } from "react";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import type { AppSettings, AppSettingsPatch } from "@/api/settings";
import { useAppSettings } from "./useAppSettings";
import "./settings.css";

const RUNNERS = [{ id: "cursor", label: "Cursor (cursor-agent CLI)" }] as const;

type FormState = {
  workerEnabled: boolean;
  runner: string;
  repoRoot: string;
  cursorBin: string;
  maxRunDurationSeconds: string;
};

function toFormState(s: AppSettings): FormState {
  return {
    workerEnabled: s.worker_enabled,
    runner: s.runner,
    repoRoot: s.repo_root,
    cursorBin: s.cursor_bin,
    maxRunDurationSeconds: String(s.max_run_duration_seconds),
  };
}

function diffPatch(initial: AppSettings, form: FormState): AppSettingsPatch {
  const out: AppSettingsPatch = {};
  if (initial.worker_enabled !== form.workerEnabled) {
    out.worker_enabled = form.workerEnabled;
  }
  if (initial.runner !== form.runner.trim()) {
    out.runner = form.runner.trim();
  }
  if (initial.repo_root !== form.repoRoot.trim()) {
    out.repo_root = form.repoRoot.trim();
  }
  if (initial.cursor_bin !== form.cursorBin.trim()) {
    out.cursor_bin = form.cursorBin.trim();
  }
  const parsedMax = Number.parseInt(form.maxRunDurationSeconds.trim() || "0", 10);
  if (Number.isFinite(parsedMax) && parsedMax !== initial.max_run_duration_seconds) {
    out.max_run_duration_seconds = parsedMax;
  }
  return out;
}

export function SettingsPage() {
  useDocumentTitle("Settings");
  const { settings, isLoading, error, patch, probe, cancelRun, refetch } =
    useAppSettings();
  const [form, setForm] = useState<FormState | null>(null);
  const [statusMsg, setStatusMsg] = useState<string>("");

  useEffect(() => {
    if (settings && form === null) {
      setForm(toFormState(settings));
    }
  }, [settings, form]);

  const isDirty = useMemo(() => {
    if (!settings || !form) return false;
    return Object.keys(diffPatch(settings, form)).length > 0;
  }, [settings, form]);

  const maxParsed = form ? Number.parseInt(form.maxRunDurationSeconds.trim() || "0", 10) : 0;
  const maxInvalid = form ? !Number.isFinite(maxParsed) || maxParsed < 0 : false;

  function handleField<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((cur) => (cur === null ? cur : { ...cur, [key]: value }));
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!settings || !form || maxInvalid) return;
    const body = diffPatch(settings, form);
    if (Object.keys(body).length === 0) return;
    setStatusMsg("");
    try {
      const next = await patch.mutateAsync(body);
      setForm(toFormState(next));
      setStatusMsg("Settings saved.");
    } catch (err) {
      setStatusMsg(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleProbe() {
    if (!form) return;
    setStatusMsg("");
    try {
      const result = await probe.mutateAsync({
        runner: form.runner.trim() || undefined,
        binary_path: form.cursorBin.trim() || undefined,
      });
      if (result.ok) {
        setStatusMsg(
          result.version
            ? `Cursor binary OK (version ${result.version}).`
            : "Cursor binary OK.",
        );
      } else {
        setStatusMsg(`Cursor binary check failed: ${result.error ?? "unknown error"}`);
      }
    } catch (err) {
      setStatusMsg(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleCancelRun() {
    setStatusMsg("");
    try {
      const result = await cancelRun.mutateAsync();
      setStatusMsg(
        result.cancelled ? "In-flight agent run cancelled." : "No agent run in flight.",
      );
    } catch (err) {
      setStatusMsg(err instanceof Error ? err.message : String(err));
    }
  }

  function handleReset() {
    if (settings) {
      setForm(toFormState(settings));
      setStatusMsg("");
    }
  }

  if (isLoading || !form || !settings) {
    return (
      <section className="settings-page" aria-busy="true">
        <h2 className="settings-page-title">Settings</h2>
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

  return (
    <section className="settings-page">
      <header className="settings-page-header">
        <h2 className="settings-page-title">Settings</h2>
        {lastUpdated ? (
          <p className="settings-page-subtitle" data-testid="settings-last-updated">
            Last updated: <time dateTime={lastUpdated}>{lastUpdated}</time>
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
          <label className="settings-field">
            <input
              type="checkbox"
              checked={form.workerEnabled}
              onChange={(e) => handleField("workerEnabled", e.target.checked)}
            />
            Enable agent worker
          </label>
          <p className="settings-field-help">
            When enabled, the worker pulls ready tasks and dispatches them to
            the configured runner.
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
            The agent worker, <code>/repo/*</code> endpoints, and{" "}
            <code>@file</code> mentions all read this path. Leave empty to
            disable repo features until you pick a workspace.
          </p>
        </fieldset>

        <fieldset className="settings-fieldset">
          <legend>Cursor binary</legend>
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
          <button
            type="button"
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
            disabled={!isDirty || patch.isPending || maxInvalid}
          >
            {patch.isPending ? "Saving…" : "Save changes"}
          </button>
          <button
            type="button"
            onClick={handleReset}
            disabled={!isDirty || patch.isPending}
          >
            Reset
          </button>
          <button
            type="button"
            onClick={() => void handleCancelRun()}
            disabled={cancelRun.isPending}
          >
            {cancelRun.isPending ? "Cancelling…" : "Cancel current run"}
          </button>
        </div>

        {statusMsg ? (
          <p
            role="status"
            data-testid="settings-status"
            className="settings-status"
          >
            {statusMsg}
          </p>
        ) : null}
      </form>
    </section>
  );
}
