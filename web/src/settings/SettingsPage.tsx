import { type FormEvent, type Dispatch, type SetStateAction, useEffect, useMemo, useState } from "react";
import { type UseMutationResult, type UseQueryResult, useQuery } from "@tanstack/react-query";
import { useLocation } from "react-router-dom";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import {
  listCursorModels,
  type AppSettings,
  type AppSettingsPatch,
  type ListCursorModelsResult,
  type ProbeCursorResult,
} from "@/api/settings";
import {
  detectBrowserTimezone,
  formatInAppTimezone,
  getTimezoneSelectOptions,
} from "@/shared/time/appTimezone";
import { useAppSettings } from "./useAppSettings";
import {
  DisplaySettingsSection,
  PhasesSettingsSection,
  RunnerSettingsSection,
  SECTION_IDS,
  SettingsActions,
  SettingsHeader,
  SettingsLoadingState,
  SettingsStatusMessage,
  WorkspaceSettingsSection,
} from "./SettingsSections";
import { UiTestModeSettingsSection } from "./UiTestModeSettingsSection";
import { SettingsNav, type SettingsNavItem } from "./SettingsNav";
import {
  SETTINGS_SUCCESS_DISMISS_MS,
  diffPatch,
  toFormState,
  type SettingsFormState,
  type SettingsStatus,
} from "./settingsForm";
import "./settings.css";

/**
 * In-page navigation rail entries. Order matches the form below
 * (workspace → runner config → phases → cosmetic → dev) so
 * the operator's vertical scroll path matches the rail's top-to-bottom
 * reading order. Keep ids in sync with `SECTION_IDS` exported from
 * SettingsSections.tsx.
 */
const SETTINGS_NAV_ITEMS: SettingsNavItem[] = [
  { id: SECTION_IDS.workspace, label: "Agent workspace" },
  { id: SECTION_IDS.cursorAgent, label: "Runner" },
  { id: SECTION_IDS.phases, label: "Phases" },
  { id: SECTION_IDS.display, label: "Display" },
  { id: SECTION_IDS.developer, label: "Developer" },
];

/**
 * Hash targets the deep-link scroll handler will honour. Includes
 * `verification` even though it is no longer a top-level nav entry —
 * legacy links keep working by landing inside the Phases section's
 * verify subgroup, which carries that id.
 */
const SETTINGS_HASH_TARGETS: ReadonlySet<string> = new Set([
  ...Object.values(SECTION_IDS),
]);

type SettingsNumericValidation = {
  maxInvalid: boolean;
  streamIdleInvalid: boolean;
  pickupInvalid: boolean;
};

type TimezoneDisplayContext = {
  browserTz: string;
  effectiveDisplayTimezone: string;
  lastUpdatedFormatted: string;
  showCustomTz: boolean;
};

function parseNonNegativeIntField(raw: string): boolean {
  const parsed = Number.parseInt(raw.trim() || "0", 10);
  return !Number.isFinite(parsed) || parsed < 0;
}

function parseSettingsNumericValidation(
  form: SettingsFormState | null,
): SettingsNumericValidation {
  if (!form) {
    return {
      maxInvalid: false,
      streamIdleInvalid: false,
      pickupInvalid: false,
    };
  }
  const pickupParsed = Number.parseInt(
    form.agentPickupDelaySeconds.trim() || "0",
    10,
  );
  const pickupInvalid =
    !Number.isFinite(pickupParsed) ||
    pickupParsed < 0 ||
    pickupParsed > 604800;
  return {
    maxInvalid: parseNonNegativeIntField(form.maxRunDurationSeconds),
    streamIdleInvalid: parseNonNegativeIntField(form.streamIdleStuckSeconds),
    pickupInvalid,
  };
}

function modelIdsFromListResponse(
  data: ListCursorModelsResult | undefined,
): Set<string> {
  if (!data?.ok || !data.models) return new Set<string>();
  return new Set(data.models.map((x) => x.id));
}

function resolveVerifyEffectiveRunner(
  form: SettingsFormState,
  settings: AppSettings,
): string {
  return (
    (form.verifyRunnerName ?? "").trim() ||
    form.runner ||
    settings.runner ||
    "cursor"
  );
}

function mergeFormAfterSettingsPatch(
  cur: SettingsFormState | null,
  formAtSubmit: SettingsFormState,
  next: AppSettings,
): SettingsFormState {
  if (cur === null) return toFormState(next);
  const merged: SettingsFormState = { ...cur };
  if (cur.runner === formAtSubmit.runner) {
    merged.runner = next.runner;
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
  if (cur.streamIdleStuckSeconds === formAtSubmit.streamIdleStuckSeconds) {
    merged.streamIdleStuckSeconds = String(next.stream_idle_stuck_seconds);
  }
  if (cur.agentPickupDelaySeconds === formAtSubmit.agentPickupDelaySeconds) {
    merged.agentPickupDelaySeconds = String(next.agent_pickup_delay_seconds);
  }
  if (cur.displayTimezone === formAtSubmit.displayTimezone) {
    merged.displayTimezone = next.display_timezone;
  }
  return merged;
}

function buildCursorProbeSuccessMessage(result: {
  binary_path?: string;
  version?: string;
}): string {
  const bits: string[] = ["Cursor binary OK"];
  if (result.binary_path) bits.push(`at ${result.binary_path}`);
  if (result.version) bits.push(`(version ${result.version})`);
  return `${bits.join(" ")}.`;
}

function resolveProbeDefaultBin(
  form: SettingsFormState,
  result: { binary_path?: string },
): string | null {
  if (result.binary_path && form.cursorBin.trim() === "") {
    return result.binary_path;
  }
  return null;
}

function computeTimezoneDisplayContext(
  form: SettingsFormState,
  lastUpdated: string,
  tzValueSet: Set<string>,
): TimezoneDisplayContext {
  const showCustomTz =
    form.displayTimezone.trim() !== "" &&
    !tzValueSet.has(form.displayTimezone.trim());
  const browserTz = detectBrowserTimezone();
  const effectiveDisplayTimezone = form.displayTimezone.trim() || browserTz;
  const lastUpdatedFormatted = lastUpdated
    ? formatInAppTimezone(lastUpdated, effectiveDisplayTimezone, {
        timeZoneName: "longOffset",
      })
    : "";
  return {
    browserTz,
    effectiveDisplayTimezone,
    lastUpdatedFormatted,
    showCustomTz,
  };
}

function useAutoDismissSettingsSuccess(
  status: SettingsStatus,
  setStatus: (status: SettingsStatus) => void,
) {
  useEffect(() => {
    if (status?.kind !== "success") return;
    const id = window.setTimeout(() => {
      setStatus(null);
    }, SETTINGS_SUCCESS_DISMISS_MS);
    return () => window.clearTimeout(id);
  }, [status, setStatus]);
}

function useSettingsSectionHashScroll(
  locationHash: string,
  isLoading: boolean,
  form: SettingsFormState | null,
  settings: AppSettings | undefined,
) {
  useEffect(() => {
    if (isLoading || !form || !settings) return;
    const hash = locationHash.replace(/^#/, "");
    if (!hash) return;
    if (!SETTINGS_HASH_TARGETS.has(hash)) return;
    const el = document.getElementById(hash);
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
  }, [isLoading, form, settings, locationHash]);
}

function useSettingsFormHydration(
  settings: AppSettings | undefined,
  form: SettingsFormState | null,
  setForm: (value: SettingsFormState | null) => void,
) {
  useEffect(() => {
    if (settings && form === null) {
      setForm(toFormState(settings));
    }
  }, [settings, form, setForm]);
}

function useSettingsCursorModelQueries(
  settings: AppSettings | undefined,
  form: SettingsFormState | null,
) {
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

  const modelIdsFromList = useMemo(
    () => modelIdsFromListResponse(cursorModelsQuery.data),
    [cursorModelsQuery.data],
  );

  const verifyEffectiveRunner =
    form && settings
      ? resolveVerifyEffectiveRunner(form, settings)
      : "cursor";

  const verifyModelsQuery = useQuery({
    queryKey: [
      "settings",
      "verify-models",
      verifyEffectiveRunner,
      form?.cursorBin,
    ],
    queryFn: ({ signal }) =>
      listCursorModels(
        {
          runner: verifyEffectiveRunner,
          binary_path: (form?.cursorBin ?? "").trim() || undefined,
        },
        { signal },
      ),
    enabled: Boolean(settings && form),
  });

  const verifyModelIdsFromList = useMemo(
    () => modelIdsFromListResponse(verifyModelsQuery.data),
    [verifyModelsQuery.data],
  );

  return {
    cursorModelsQuery,
    modelIdsFromList,
    verifyModelsQuery,
    verifyModelIdsFromList,
  };
}

function settingsErrorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function createSettingsFieldHandler(
  setForm: Dispatch<SetStateAction<SettingsFormState | null>>,
  resolvedDefaultBin: string | null,
  setResolvedDefaultBin: Dispatch<SetStateAction<string | null>>,
) {
  return function handleField<K extends keyof SettingsFormState>(
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
  };
}

async function submitSettingsForm(params: {
  settings: AppSettings;
  form: SettingsFormState;
  numericValidation: SettingsNumericValidation;
  patch: UseMutationResult<AppSettings, Error, AppSettingsPatch, unknown>;
  setForm: Dispatch<SetStateAction<SettingsFormState | null>>;
  setStatus: Dispatch<SetStateAction<SettingsStatus>>;
}): Promise<void> {
  const { settings, form, numericValidation, patch, setForm, setStatus } =
    params;
  const { maxInvalid, streamIdleInvalid, pickupInvalid } = numericValidation;
  if (maxInvalid || streamIdleInvalid || pickupInvalid) return;
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
  // `useTaskDeleteFlow` — capture the
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
    setForm((cur) => mergeFormAfterSettingsPatch(cur, formAtSubmit, next));
    setStatus({ kind: "success", message: "Settings saved." });
  } catch (err) {
    setStatus({ kind: "error", message: settingsErrorMessage(err) });
  }
}

async function probeCursorBinary(params: {
  form: SettingsFormState;
  probe: UseMutationResult<
    ProbeCursorResult,
    Error,
    { runner?: string; binary_path?: string },
    unknown
  >;
  setStatus: Dispatch<SetStateAction<SettingsStatus>>;
  setResolvedDefaultBin: Dispatch<SetStateAction<string | null>>;
}): Promise<void> {
  const { form, probe, setStatus, setResolvedDefaultBin } = params;
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
      setStatus({
        kind: "success",
        message: buildCursorProbeSuccessMessage(result),
      });
      setResolvedDefaultBin(resolveProbeDefaultBin(form, result));
      return;
    }
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
  } catch (err) {
    setStatus({ kind: "error", message: settingsErrorMessage(err) });
  }
}

type SettingsPageLoadedViewProps = {
  form: SettingsFormState;
  status: SettingsStatus;
  resolvedDefaultBin: string | null;
  isDirty: boolean;
  numericValidation: SettingsNumericValidation;
  tzSelectOptions: ReturnType<typeof getTimezoneSelectOptions>;
  browserTz: string;
  showCustomTz: boolean;
  lastUpdated: string;
  lastUpdatedFormatted: string;
  cursorModelsQuery: UseQueryResult<ListCursorModelsResult, Error>;
  modelIdsFromList: Set<string>;
  verifyModelsQuery: UseQueryResult<ListCursorModelsResult, Error>;
  verifyModelIdsFromList: Set<string>;
  patchPending: boolean;
  probePending: boolean;
  onField: ReturnType<typeof createSettingsFieldHandler>;
  onSubmit: (e: FormEvent) => void;
  onProbe: () => void;
  onDiscard: () => void;
};

function SettingsPageLoadedView({
  form,
  status,
  resolvedDefaultBin,
  isDirty,
  numericValidation,
  tzSelectOptions,
  browserTz,
  showCustomTz,
  lastUpdated,
  lastUpdatedFormatted,
  cursorModelsQuery,
  modelIdsFromList,
  verifyModelsQuery,
  verifyModelIdsFromList,
  patchPending,
  probePending,
  onField,
  onSubmit,
  onProbe,
  onDiscard,
}: SettingsPageLoadedViewProps) {
  const { maxInvalid, streamIdleInvalid, pickupInvalid } = numericValidation;

  return (
    <section className="settings-page">
      <SettingsHeader
        lastUpdated={lastUpdated}
        lastUpdatedFormatted={lastUpdatedFormatted}
      />

      <div className="settings-layout">
        <aside className="settings-layout-aside">
          <SettingsNav items={SETTINGS_NAV_ITEMS} />
        </aside>

        <form className="settings-form" onSubmit={onSubmit}>
          <WorkspaceSettingsSection />

          <RunnerSettingsSection
            form={form}
            resolvedDefaultBin={resolvedDefaultBin}
            probePending={probePending}
            onField={onField}
            onProbe={onProbe}
          />

          <PhasesSettingsSection
            form={form}
            pickupInvalid={pickupInvalid}
            maxInvalid={maxInvalid}
            streamIdleInvalid={streamIdleInvalid}
            cursorModelsQuery={cursorModelsQuery}
            modelIdsFromList={modelIdsFromList}
            verifyModelsQuery={verifyModelsQuery}
            verifyModelIdsFromList={verifyModelIdsFromList}
            onField={onField}
          />

          <DisplaySettingsSection
            form={form}
            browserTz={browserTz}
            options={tzSelectOptions}
            showCustomTz={showCustomTz}
            onField={onField}
          />

          <UiTestModeSettingsSection />

          <SettingsStatusMessage status={status} />

          <SettingsActions
            isDirty={isDirty}
            maxInvalid={maxInvalid}
            streamIdleInvalid={streamIdleInvalid}
            pickupInvalid={pickupInvalid}
            patchPending={patchPending}
            onDiscard={onDiscard}
          />
        </form>
      </div>
    </section>
  );
}

function useSettingsPageController() {
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

  useAutoDismissSettingsSuccess(status, setStatus);
  useSettingsSectionHashScroll(
    location.hash,
    isLoading,
    form,
    settings,
  );
  useSettingsFormHydration(settings, form, setForm);

  const isDirty = useMemo(() => {
    if (!settings || !form) return false;
    return Object.keys(diffPatch(settings, form)).length > 0;
  }, [settings, form]);

  const {
    cursorModelsQuery,
    modelIdsFromList,
    verifyModelsQuery,
    verifyModelIdsFromList,
  } = useSettingsCursorModelQueries(settings, form);

  const tzSelectOptions = useMemo(() => getTimezoneSelectOptions(), []);
  const tzValueSet = useMemo(
    () => new Set(tzSelectOptions.map((o) => o.value)),
    [tzSelectOptions],
  );

  const numericValidation = parseSettingsNumericValidation(form);
  const handleField = createSettingsFieldHandler(
    setForm,
    resolvedDefaultBin,
    setResolvedDefaultBin,
  );

  const loadedViewProps = useMemo((): SettingsPageLoadedViewProps | null => {
    if (isLoading || !form || !settings) return null;
    const lastUpdated = settings.updated_at ?? "";
    const {
      browserTz,
      lastUpdatedFormatted,
      showCustomTz,
    } = computeTimezoneDisplayContext(
      form,
      lastUpdated,
      tzValueSet,
    );
    return {
      form,
      status,
      resolvedDefaultBin,
      isDirty,
      numericValidation,
      tzSelectOptions,
      browserTz,
      showCustomTz,
      lastUpdated,
      lastUpdatedFormatted,
      cursorModelsQuery,
      modelIdsFromList,
      verifyModelsQuery,
      verifyModelIdsFromList,
      patchPending: patch.isPending,
      probePending: probe.isPending,
      onField: handleField,
      onSubmit: (e) => {
        e.preventDefault();
        void submitSettingsForm({
          settings,
          form,
          numericValidation,
          patch,
          setForm,
          setStatus,
        });
      },
      onProbe: () => {
        void probeCursorBinary({
          form,
          probe,
          setStatus,
          setResolvedDefaultBin,
        });
      },
      onDiscard: () => setForm(toFormState(settings)),
    };
  }, [
    isLoading,
    form,
    settings,
    status,
    resolvedDefaultBin,
    isDirty,
    numericValidation,
    tzSelectOptions,
    tzValueSet,
    cursorModelsQuery,
    modelIdsFromList,
    verifyModelsQuery,
    verifyModelIdsFromList,
    patch,
    probe,
    handleField,
  ]);

  return {
    error,
    refetch,
    loadedViewProps,
  };
}

export function SettingsPage() {
  useDocumentTitle("Settings");
  const { error, refetch, loadedViewProps } = useSettingsPageController();

  if (!loadedViewProps) {
    return (
      <SettingsLoadingState
        error={error}
        onRetry={() => {
          void refetch();
        }}
      />
    );
  }

  return <SettingsPageLoadedView {...loadedViewProps} />;
}
