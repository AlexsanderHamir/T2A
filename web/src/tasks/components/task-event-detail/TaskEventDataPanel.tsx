import { useId, useState } from "react";
import ReactMarkdown from "react-markdown";
import { CopyableId } from "@/shared/CopyableId";
import type { TaskEventType } from "@/types/task";
import {
  parsePhaseEventOverview,
  type PhaseEventOverviewModel,
} from "../../task-events/parsePhaseEventOverview";

function formatDurationMs(ms: number | undefined): string | undefined {
  if (ms === undefined) return undefined;
  if (ms < 1000) return `${ms} ms`;
  const s = ms / 1000;
  if (s < 60) return `${s >= 10 ? Math.round(s) : s.toFixed(1)} s`;
  const m = Math.floor(s / 60);
  const rem = Math.round(s % 60);
  return `${m}m ${rem}s`;
}

function formatTokens(n: number | undefined): string {
  if (n === undefined) return "—";
  return n.toLocaleString();
}

function statusTone(
  status: string,
): "success" | "failed" | "neutral" {
  const s = status.toLowerCase();
  if (s === "succeeded" || s === "skipped") return "success";
  if (s === "failed") return "failed";
  return "neutral";
}

function PhaseEventOverviewBody({ model }: { model: PhaseEventOverviewModel }) {
  const summaryText = model.summary?.replace(/^\n+/, "").trimEnd();
  const tone = statusTone(model.status);
  const hasUsage =
    model.usage &&
    (model.usage.inputTokens !== undefined ||
      model.usage.outputTokens !== undefined ||
      model.usage.cacheReadTokens !== undefined ||
      model.usage.cacheWriteTokens !== undefined);

  return (
    <div className="task-event-phase-overview">
      <div className="task-event-phase-overview-header">
        <span
          className="task-event-phase-pill"
          data-phase={model.phase.toLowerCase()}
        >
          {model.phase}
        </span>
        <span
          className="task-event-status-pill"
          data-tone={tone}
          data-status={model.status.toLowerCase()}
        >
          {model.status}
        </span>
      </div>

      <dl className="task-event-phase-meta">
        {model.cycleId ? (
          <div>
            <dt>Cycle</dt>
            <dd>
              <CopyableId value={model.cycleId} />
            </dd>
          </div>
        ) : null}
        {model.phaseSeq !== undefined ? (
          <div>
            <dt>Phase seq</dt>
            <dd>{model.phaseSeq}</dd>
          </div>
        ) : null}
        {(model.durationMs !== undefined ||
          model.durationApiMs !== undefined) && (
          <div>
            <dt>Duration</dt>
            <dd>
              {formatDurationMs(model.durationMs ?? model.durationApiMs)}
              {model.durationMs !== undefined &&
              model.durationApiMs !== undefined &&
              model.durationMs !== model.durationApiMs ? (
                <span className="task-event-phase-duration-note">
                  {" "}
                  (API {formatDurationMs(model.durationApiMs)})
                </span>
              ) : null}
            </dd>
          </div>
        )}
        {model.requestId ? (
          <div>
            <dt>Request</dt>
            <dd>
              <CopyableId value={model.requestId} />
            </dd>
          </div>
        ) : null}
        {model.sessionId ? (
          <div>
            <dt>Session</dt>
            <dd>
              <CopyableId value={model.sessionId} />
            </dd>
          </div>
        ) : null}
      </dl>

      {hasUsage ? (
        <div className="task-event-usage-card" aria-label="Token usage">
          <h4 className="task-event-usage-heading">Usage</h4>
          <div className="task-event-usage-grid">
            <div>
              <span className="task-event-usage-label">Input</span>
              <span className="task-event-usage-value">
                {formatTokens(model.usage?.inputTokens)}
              </span>
            </div>
            <div>
              <span className="task-event-usage-label">Output</span>
              <span className="task-event-usage-value">
                {formatTokens(model.usage?.outputTokens)}
              </span>
            </div>
            <div>
              <span className="task-event-usage-label">Cache read</span>
              <span className="task-event-usage-value">
                {formatTokens(model.usage?.cacheReadTokens)}
              </span>
            </div>
            <div>
              <span className="task-event-usage-label">Cache write</span>
              <span className="task-event-usage-value">
                {formatTokens(model.usage?.cacheWriteTokens)}
              </span>
            </div>
          </div>
        </div>
      ) : null}

      {(model.failureKind || model.standardizedMessage || model.stderrTail) && (
        <div
          className="task-event-phase-alert"
          role="alert"
          data-severity="error"
        >
          {model.standardizedMessage ? (
            <p className="task-event-phase-alert-msg">
              {model.standardizedMessage}
            </p>
          ) : null}
          {model.failureKind ? (
            <p className="task-event-phase-alert-kind">
              <span className="muted">Kind:</span>{" "}
              <code>{model.failureKind}</code>
            </p>
          ) : null}
          {model.stderrTail ? (
            <pre className="task-event-phase-stderr">{model.stderrTail}</pre>
          ) : null}
        </div>
      )}

      {summaryText ? (
        <div className="task-event-summary-block">
          <h4 className="task-event-summary-heading">Summary</h4>
          <div className="task-event-markdown">
            <ReactMarkdown>{summaryText}</ReactMarkdown>
          </div>
        </div>
      ) : null}
    </div>
  );
}

type TabId = "overview" | "json";

export function TaskEventDataPanel({
  eventType,
  data,
}: {
  eventType: TaskEventType;
  data: Record<string, unknown>;
}) {
  const overview = parsePhaseEventOverview(eventType, data);
  const dataJson = JSON.stringify(data, null, 2);
  const baseId = useId();
  const tabOverviewId = `${baseId}-tab-overview`;
  const tabJsonId = `${baseId}-tab-json`;
  const panelOverviewId = `${baseId}-panel-overview`;
  const panelJsonId = `${baseId}-panel-json`;

  const [tab, setTab] = useState<TabId>(() =>
    overview ? "overview" : "json",
  );

  const jsonPre = (
    <pre className="task-timeline-data task-event-detail-data-pre">
      {dataJson}
    </pre>
  );

  return (
    <div className="task-event-detail-data-block">
      <h3 className="task-detail-subheading task-event-data-heading">
        <span>Event data</span>
      </h3>

      {overview ? (
        <>
          <div
            className="task-event-data-tabs"
            role="tablist"
            aria-label="Event payload view"
          >
            <button
              type="button"
              id={tabOverviewId}
              role="tab"
              aria-selected={tab === "overview"}
              aria-controls={panelOverviewId}
              tabIndex={tab === "overview" ? 0 : -1}
              className="task-event-data-tab"
              data-active={tab === "overview" ? "true" : undefined}
              onClick={() => setTab("overview")}
            >
              Overview
            </button>
            <button
              type="button"
              id={tabJsonId}
              role="tab"
              aria-selected={tab === "json"}
              aria-controls={panelJsonId}
              tabIndex={tab === "json" ? 0 : -1}
              className="task-event-data-tab"
              data-active={tab === "json" ? "true" : undefined}
              onClick={() => setTab("json")}
            >
              Raw JSON
            </button>
          </div>
          <div
            id={panelOverviewId}
            role="tabpanel"
            aria-labelledby={tabOverviewId}
            hidden={tab !== "overview"}
            className="task-event-data-panel"
          >
            <PhaseEventOverviewBody model={overview} />
          </div>
          <div
            id={panelJsonId}
            role="tabpanel"
            aria-labelledby={tabJsonId}
            hidden={tab !== "json"}
            className="task-event-data-panel"
          >
            {jsonPre}
          </div>
        </>
      ) : (
        jsonPre
      )}
    </div>
  );
}
