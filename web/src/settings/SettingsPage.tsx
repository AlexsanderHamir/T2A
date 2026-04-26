import { type FormEvent, useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useLocation } from "react-router-dom";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { listCursorModels } from "@/api/settings";
import {
  detectBrowserTimezone,
  formatInAppTimezone,
  getTimezoneSelectOptions,
} from "@/shared/time/appTimezone";
import { useAppSettings } from "./useAppSettings";
import {
  AgentWorkerSettingsSection,
  CursorAgentSettingsSection,
  DisplaySettingsSection,
  RunTimeoutSettingsSection,
  SettingsActions,
  SettingsHeader,
  SettingsLoadingState,
  SettingsStatusMessage,
  WorkspaceSettingsSection,
  WorkspaceWarning,
} from "./SettingsSections";
import {
  SETTINGS_SUCCESS_DISMISS_MS,
  diffPatch,
  toFormState,
  type SettingsFormState,
  type SettingsStatus,
} from "./settingsForm";
import "./settings.css";

export function SettingsPage() {
  useDocumentTitle("Settings");
  const location = useLocation();
  const { settings, isLoading, error, patch, probe, refetch } =
    useAppSettings();
  const [form, setForm] = useState<SettingsFormState | null>(null);
  const [status, setStatus] = useState<SettingsStatus>(null);
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
    if (status?.kind !== "success") return;
    const id = window.setTimeout(() => {
      setStatus(null);
    }, SETTINGS_SUCCESS_DISMISS_MS);
    return () => window.clearTimeout(id);
  }, [status]);

  /**
   * Client navigations to `/settings#cursor-agent` do not scroll the way a full
   * page load would, and the `#cursor-agent` target is not in the DOM until
   * settings finish loading — scroll after the form mounts.
   */
  useEffect(() => {
    if (isLoading || !form || !settings) return;
    if (location.hash !== "#cursor-agent") return;
    const el = document.getElementById("cursor-agent");
    if (!el) return;
    const prefersReduced =
      typeof window.matchMedia === "function" &&
      window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const run = () => {
      el.scrollIntoView({
        behavior: prefersReduced ? "auto" : "smooth",
        block: "start",
      });
    };
    requestAnimationFrame(() => {
      requestAnimationFrame(run);
    });
  }, [isLoading, form, settings, location.hash]);

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

  const tzSelectOptions = useMemo(() => getTimezoneSelectOptions(), []);
  const tzValueSet = useMemo(
    () => new Set(tzSelectOptions.map((o) => o.value)),
    [tzSelectOptions],
  );

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

  function handleField<K extends keyof SettingsFormState>(
    key: K,
    value: SettingsFormState[K],
  ) {
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
        const merged: SettingsFormState = { ...cur };
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
      <SettingsLoadingState
        error={error}
        onRetry={() => {
          void refetch();
        }}
      />
    );
  }

  const repoRootEmpty = form.repoRoot.trim() === "";
  const lastUpdated = settings.updated_at ?? "";
  // Tolerate operator-pasted custom zones that aren't in the
  // Intl.supportedValuesOf list — show them as a synthetic option so
  // the <select> can display the saved value rather than silently
  // falling back to the first list item.
  const showCustomTz =
    form.displayTimezone.trim() !== "" &&
    !tzValueSet.has(form.displayTimezone.trim());
  // Browser-detected zone surfaced in the "Auto-detect" option label so
  // operators can see WHICH zone auto-detect will resolve to before
  // committing. Recomputed per-render — detectBrowserTimezone is a
  // single Intl.DateTimeFormat() call, negligible cost.
  const browserTz = detectBrowserTimezone();
  // Same effective zone as the timezone <select>: explicit IANA, or
  // browser when Auto-detect (empty). Trim so whitespace-only never
  // bypasses auto-detect or slips an invalid zone to Intl.
  const effectiveDisplayTimezone = form.displayTimezone.trim() || browserTz;
  // longOffset aligns the suffix with Meet-style menu labels (GMT±hh:mm)
  // instead of a separate abbreviation (e.g. PDT) that looks mismatched.
  const lastUpdatedFormatted = lastUpdated
    ? formatInAppTimezone(lastUpdated, effectiveDisplayTimezone, {
        timeZoneName: "longOffset",
      })
    : "";

  return (
    <section className="settings-page">
      <SettingsHeader
        lastUpdated={lastUpdated}
        lastUpdatedFormatted={lastUpdatedFormatted}
      />

      {repoRootEmpty ? <WorkspaceWarning /> : null}

      <form className="settings-form" onSubmit={(e) => void handleSubmit(e)}>
        <AgentWorkerSettingsSection
          form={form}
          pickupInvalid={pickupInvalid}
          onField={handleField}
        />

        <DisplaySettingsSection
          form={form}
          browserTz={browserTz}
          options={tzSelectOptions}
          showCustomTz={showCustomTz}
          onField={handleField}
        />

        <WorkspaceSettingsSection form={form} onField={handleField} />

        <CursorAgentSettingsSection
          form={form}
          cursorModelsQuery={cursorModelsQuery}
          modelIdsFromList={modelIdsFromList}
          resolvedDefaultBin={resolvedDefaultBin}
          probePending={probe.isPending}
          onField={handleField}
          onProbe={() => {
            void handleProbe();
          }}
        />

        <RunTimeoutSettingsSection
          form={form}
          maxInvalid={maxInvalid}
          onField={handleField}
        />

        <SettingsActions
          isDirty={isDirty}
          maxInvalid={maxInvalid}
          pickupInvalid={pickupInvalid}
          patchPending={patch.isPending}
          onDiscard={() => setForm(toFormState(settings))}
        />

        <SettingsStatusMessage status={status} />
      </form>
    </section>
  );
}
