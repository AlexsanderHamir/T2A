import { useMemo } from "react";
import { SettingsSelect, type SettingsSelectOption } from "../SettingsSelect";
import { RUNNERS, type SettingsFormState } from "../settingsForm";
import { SECTION_IDS } from "./sectionIds";
import { SectionCard } from "./settingsSectionLayout";
import type { HandleField } from "./settingsSectionTypes";

/**
 * Runner — the agent CLI used by every phase. Today only `cursor` is
 * registered, so the picker is single-option, but the section is
 * framed generically so adding a runner later is a matter of pushing
 * a new entry into RUNNERS without rewriting the UI.
 *
 * Holds the runner-level configuration that is independent of any
 * single phase: the runner identity, the binary path, and a probe to
 * verify it works. Per-phase choices (which model to pass for execute
 * vs verify) live under the Phases section. This mirrors the backend:
 * `cursor_bin` is a runner setting (shared across phases), while
 * `cursor_model` and `verify_runner_model` are phase-keyed model
 * overrides handed to that one runner.
 *
 * The card retains `id="cursor-agent"` so legacy deep links from
 * TaskModelConfigModal still land on a meaningful target.
 */
export function RunnerSettingsSection({
  form,
  resolvedDefaultBin,
  probePending,
  onField,
  onProbe,
}: {
  form: SettingsFormState;
  resolvedDefaultBin: string | null;
  probePending: boolean;
  onField: HandleField;
  onProbe: () => void;
}) {
  const runnerOptions = useMemo((): SettingsSelectOption[] => {
    const opts: SettingsSelectOption[] = RUNNERS.map((r) => ({
      value: r.id,
      label: r.label,
    }));
    const saved = form.runner.trim();
    if (saved !== "" && !RUNNERS.some((r) => r.id === saved)) {
      opts.push({ value: saved, label: `${saved} (saved — not registered)` });
    }
    return opts;
  }, [form.runner]);

  return (
    <SectionCard id={SECTION_IDS.cursorAgent} title="Runner">
      <label className="settings-field">
        <span className="settings-field-label">Runner</span>
        <SettingsSelect
          testId="settings-runner-select"
          value={form.runner}
          onChange={(next) => onField("runner", next)}
          options={runnerOptions}
          searchable={false}
        />
      </label>
      <p className="settings-field-help">
        The agent CLI used by every phase. More runners coming soon.
      </p>

      <label className="settings-field">
        <span className="settings-field-label">CLI path</span>
        <input
          type="text"
          value={form.cursorBin}
          onChange={(e) => onField("cursorBin", e.target.value)}
          placeholder="/usr/local/bin/cursor-agent"
          spellCheck={false}
          autoComplete="off"
        />
      </label>
      <p className="settings-field-help">
        Empty = auto-detect on PATH. Test before saving.
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
      <div className="settings-inline-actions">
        <button
          type="button"
          className="settings-btn settings-btn--secondary"
          onClick={onProbe}
          disabled={probePending}
        >
          {probePending ? "Testing…" : "Test binary"}
        </button>
      </div>
    </SectionCard>
  );
}
