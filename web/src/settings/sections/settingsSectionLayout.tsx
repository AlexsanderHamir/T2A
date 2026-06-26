import { type UseQueryResult } from "@tanstack/react-query";
import { useMemo } from "react";
import {
  filterCursorModelsForSelect,
  normalizeCursorModelSelectValue,
} from "@/api/cursorModels";
import type { ListCursorModelsResult } from "@/api/settings";
import { SettingsSelect, groupModelSelectRows, type SettingsSelectOption } from "../SettingsSelect";

/**
 * Section card scaffold. Renders a real `<h2>` heading inside an
 * accessible region so the page has proper outline structure for
 * screen readers and the in-page nav. The previous inset `<legend>`
 * pattern was visually decorative chrome; promoting it to an h2
 * gives the section a real anchor a sighted operator can scan and
 * an assistive-tech user can land on.
 */
export function SectionCard({
  id,
  title,
  children,
}: {
  id: string;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section
      id={id}
      className="settings-section"
      aria-labelledby={`${id}-title`}
    >
      <h2 id={`${id}-title`} className="settings-section-title">
        {title}
      </h2>
      <div className="settings-section-body">{children}</div>
    </section>
  );
}

/**
 * Reusable model picker for a phase. Centralises the "Auto + filtered
 * runner models + saved-but-unknown synthetic option + loading and
 * error inline reporting" pattern shared by the execute and verify
 * phase blocks. Both phases drive the same cursor-agent binary, so
 * the wire shape is identical; the only thing that varies per call
 * site is which form field the value writes back to and which
 * react-query observable feeds the option list.
 */
export function PhaseModelField({
  testId,
  value,
  onChange,
  query,
  knownIds,
  disabled,
}: {
  testId: string;
  value: string;
  onChange: (next: string) => void;
  query: UseQueryResult<ListCursorModelsResult, Error>;
  knownIds: Set<string>;
  disabled?: boolean;
}) {
  const selectValue = normalizeCursorModelSelectValue(value);
  const models = filterCursorModelsForSelect(
    query.data?.ok ? query.data.models : undefined,
  );
  const modelOptions = useMemo((): SettingsSelectOption[] => {
    const opts: SettingsSelectOption[] = [{ value: "", label: "Auto" }];
    for (const m of models) {
      opts.push({ value: m.id, label: m.label });
    }
    if (selectValue !== "" && !knownIds.has(selectValue)) {
      opts.push({
        value: selectValue,
        label: `${selectValue} (saved — not in current list)`,
      });
    }
    return opts;
  }, [models, selectValue, knownIds]);

  const modelRows = useMemo(
    () => groupModelSelectRows(modelOptions),
    [modelOptions],
  );

  return (
    <>
      <label className="settings-field">
        <span className="settings-field-label">Model</span>
        <SettingsSelect
          testId={testId}
          value={selectValue}
          onChange={onChange}
          options={modelOptions}
          rows={modelRows}
          disabled={disabled || query.isFetching}
          ariaBusy={query.isFetching}
          searchPlaceholder="Search models…"
        />
      </label>
      {query.isError ? (
        <p role="alert" className="settings-field-error">
          Could not load models for this runner:{" "}
          {query.error instanceof Error
            ? query.error.message
            : String(query.error)}
        </p>
      ) : null}
      {query.data && !query.data.ok ? (
        <p role="alert" className="settings-field-error">
          {query.data.error ?? "Model list failed."}
        </p>
      ) : null}
    </>
  );
}

/**
 * Phase icon — outline SVG matching the icon weight used elsewhere
 * in the app (1.6px stroke, 18px viewport). Execute shows a power
 * bolt (active work); Verify shows a shield-check (judgment pass).
 * Inline rather than imported from an icon library so the settings
 * page does not pull a new dependency for two glyphs.
 */
function PhaseIcon({ phase }: { phase: "execute" | "verify" }) {
  if (phase === "execute") {
    return (
      <svg
        className="settings-phase-icon"
        viewBox="0 0 24 24"
        width="18"
        height="18"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d="M13 3 4.5 13.5h6L11 21l8.5-10.5h-6L13 3Z" />
      </svg>
    );
  }
  return (
    <svg
      className="settings-phase-icon"
      viewBox="0 0 24 24"
      width="18"
      height="18"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M12 3 4 6v6c0 4.5 3.5 8 8 9 4.5-1 8-4.5 8-9V6l-8-3Z" />
      <path d="m9 12 2 2 4-4" />
    </svg>
  );
}

/**
 * Phase flow connector — small vertical chevron + "then" label
 * rendered between the Execute and Verify panels so the lifecycle
 * reads as a sequence, not two coequal cards. The arrow is purely
 * decorative; the screen-reader-only label is "then" so AT users
 * still hear the ordering.
 */
export function PhaseFlowConnector() {
  return (
    <div className="settings-phase-flow" aria-hidden="true">
      <span className="settings-phase-flow-line" />
      <span className="settings-phase-flow-label">then</span>
      <svg
        className="settings-phase-flow-icon"
        viewBox="0 0 12 12"
        width="12"
        height="12"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M6 2v8" />
        <path d="m3 7 3 3 3-3" />
      </svg>
    </div>
  );
}

/**
 * Nested phase panel — Execute and Verify each get their own inset
 * card with a phase icon and name so the operator can scan the
 * lifecycle without reading every field label.
 */
export function PhasePanel({
  id,
  phase,
  description,
  children,
}: {
  id: string;
  phase: "execute" | "verify";
  description: string;
  children: React.ReactNode;
}) {
  const title = phase === "execute" ? "Execute" : "Verify";
  return (
    <section
      id={id}
      className="settings-phase-panel"
      data-phase={phase}
      aria-labelledby={`${id}-title`}
    >
      <header className="settings-phase-panel-header">
        <span className="settings-phase-panel-glyph" aria-hidden="true">
          <PhaseIcon phase={phase} />
        </span>
        <div className="settings-phase-panel-heading">
          <h3
            id={`${id}-title`}
            className="settings-phase-panel-title"
          >
            {title}
          </h3>
          <p className="settings-phase-panel-desc">{description}</p>
        </div>
      </header>
      <div className="settings-phase-panel-body">{children}</div>
    </section>
  );
}

export function PhaseFieldGroup({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="settings-phase-group">
      <p className="settings-phase-group-title">{title}</p>
      {children}
    </div>
  );
}
