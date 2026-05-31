import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import type { AppSettings, ListCursorModelsResult } from "@/api/settings";
import { settingsQueryKeys } from "@/tasks/task-query/queryKeys";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import { APP_SETTINGS_DEFAULTS } from "@/test/settingsDefaults";
import { TaskCreateModal } from "./TaskCreateModal";

const testAppSettings: AppSettings = {
  ...APP_SETTINGS_DEFAULTS,
  ...TASK_TEST_DEFAULTS,
  worker_enabled: false,
  optimistic_mutations_enabled: false,
};

const testCursorModelsEmpty: ListCursorModelsResult = {
  ok: true,
  runner: TASK_TEST_DEFAULTS.runner,
  models: [],
};

function renderModal(props?: Partial<ComponentProps<typeof TaskCreateModal>>) {
  const base: ComponentProps<typeof TaskCreateModal> = {
    pending: false,
    saving: false,
    draftSaving: false,
    draftSaveLabel: null,
    draftSaveError: false,
    onClose: vi.fn(),
    title: "Draft title",
    prompt: "Draft prompt",
    priority: "medium",
    taskType: "general",
    checklistItems: [],
    onTitleChange: vi.fn(),
    onPromptChange: vi.fn(),
    onPriorityChange: vi.fn(),
    onTaskTypeChange: vi.fn(),
    onAppendChecklistCriterion: vi.fn(),
    onUpdateChecklistRow: vi.fn(),
    onRemoveChecklistRow: vi.fn(),
    pendingSubtasks: [],
    onAddPendingSubtask: vi.fn(),
    onUpdatePendingSubtask: vi.fn(),
    onRemovePendingSubtask: vi.fn(),
    evaluatePending: false,
    evaluation: null,
    dmapCommitLimit: "5",
    dmapDomain: "",
    dmapDescription: "",
    onDmapCommitLimitChange: vi.fn(),
    onDmapDomainChange: vi.fn(),
    onDmapDescriptionChange: vi.fn(),
    taskRunner: TASK_TEST_DEFAULTS.runner,
    taskCursorModel: TASK_TEST_DEFAULTS.cursor_model,
    onTaskRunnerChange: vi.fn(),
    onTaskCursorModelChange: vi.fn(),
    schedule: null,
    onScheduleChange: vi.fn(),
    autonomyEnabled: true,
    onAutonomyChange: vi.fn(),
    tagsCsv: "",
    milestone: "",
    projectId: "",
    dependsOn: [],
    onTagsCsvChange: vi.fn(),
    onMilestoneChange: vi.fn(),
    onDependsOnChange: vi.fn(),
    appTimezone: "UTC",
    onSaveDraft: vi.fn(),
    onEvaluate: vi.fn(),
    onSubmit: vi.fn(),
  };
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity },
    },
  });
  client.setQueryData(settingsQueryKeys.app(), testAppSettings);
  client.setQueryData(
    [...settingsQueryKeys.all, "create-modal-cursor-models", "cursor", ""],
    testCursorModelsEmpty,
  );
  return render(
    <QueryClientProvider client={client}>
      <TaskCreateModal {...base} {...props} />
    </QueryClientProvider>,
  );
}

/** Expand the "More options" disclosure so tests can interact with the
 *  agent + schedule + metadata controls it collapses by default. */
async function expandMoreOptions(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByTestId("task-create-more-options-toggle"));
}

describe("TaskCreateModal", () => {
  it("shows Evaluate action and calls onEvaluate", async () => {
    const user = userEvent.setup();
    const onEvaluate = vi.fn();
    renderModal({ onEvaluate });
    await user.click(screen.getByRole("button", { name: /^evaluate$/i }));
    expect(onEvaluate).toHaveBeenCalledTimes(1);
  });

  it("renders evaluation summary when available", () => {
    renderModal({
      evaluation: {
        overallScore: 86,
        overallSummary: "Strong draft, likely ready for creation.",
        sections: [
          { key: "title", score: 90 },
          { key: "initial_prompt", score: 84 },
        ],
      },
    });
    const panel = screen.getByRole("region", {
      name: /draft evaluation summary/i,
    });
    expect(
      within(panel).getByRole("heading", { name: /latest evaluation score/i }),
    ).toBeInTheDocument();
    expect(within(panel).getByText(/86/i)).toBeInTheDocument();
    expect(within(panel).getByText(/title/i)).toBeInTheDocument();
  });

  it("keeps the evaluation live region mounted but visually silent before any score", () => {
    renderModal({ evaluation: null });
    // Region stays in the DOM so the first evaluation result is
    // announced by assistive tech, but it renders no visible chrome
    // (the Evaluate button in the footer is the affordance, not a
    // boxed empty-state panel).
    const panel = screen.getByRole("region", {
      name: /draft evaluation summary/i,
    });
    expect(panel).toBeEmptyDOMElement();
    expect(panel).toHaveClass("task-create-evaluation-summary--empty");
  });

  it("shows Save draft action and calls onSaveDraft", async () => {
    const user = userEvent.setup();
    const onSaveDraft = vi.fn();
    renderModal({ onSaveDraft });
    await user.click(screen.getByRole("button", { name: /save draft/i }));
    expect(onSaveDraft).toHaveBeenCalledTimes(1);
  });

  it("disables Save draft while draft save is pending", () => {
    renderModal({ draftSaving: true });
    expect(
      screen.getByRole("button", { name: /saving draft/i }),
    ).toBeDisabled();
  });

  it("does not render a parent task picker (subtasks are created from the parent task page)", () => {
    renderModal();
    expect(
      screen.queryByRole("combobox", { name: /parent task/i }),
    ).not.toBeInTheDocument();
    expect(
      document.querySelector(".task-create-parent-loading"),
    ).toBeNull();
    expect(
      screen.queryByText(/inherit parent's checklist criteria/i),
    ).not.toBeInTheDocument();
  });

  it("always titles the modal 'New task' (the modal no longer creates subtasks)", () => {
    renderModal();
    expect(
      screen.getByRole("heading", { name: /^new task$/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: /^new subtask$/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /^create$/i }),
    ).toBeInTheDocument();
  });

  it("renders a SchedulePicker with the immediate-pickup caption when no schedule is set", async () => {
    const user = userEvent.setup();
    renderModal();
    await expandMoreOptions(user);
    expect(
      screen.getByText(/picks up immediately when the worker is free/i),
    ).toBeInTheDocument();
    expect(screen.getByTestId("schedule-picker-input")).toBeInTheDocument();
  });

  it("forwards quick-pick selections to onScheduleChange", async () => {
    const user = userEvent.setup();
    const onScheduleChange = vi.fn();
    renderModal({ onScheduleChange, appTimezone: "UTC" });
    await expandMoreOptions(user);
    // The new picker funnels every preset through an anchored popover; open
    // it first, then click the +1 hour chip to verify the modal forwards the
    // emitted ISO upward unchanged.
    await user.click(screen.getByTestId("schedule-picker-quick-trigger"));
    await user.click(screen.getByTestId("schedule-picker-quick-hour-1"));
    expect(onScheduleChange).toHaveBeenCalledTimes(1);
    const v = onScheduleChange.mock.calls[0][0];
    expect(typeof v).toBe("string");
    expect(v).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/);
  });

  it("renders the picker caption in the chosen app timezone when a schedule is set", async () => {
    const user = userEvent.setup();
    renderModal({
      schedule: "2026-04-19T13:00:00Z",
      appTimezone: "America/New_York",
    });
    await expandMoreOptions(user);
    // 13:00Z = 09:00 EDT in April.
    expect(screen.getByText(/agent will pick up at/i)).toBeInTheDocument();
    expect(screen.getByText(/09:00/)).toBeInTheDocument();
  });

  it("does not render mutation error callouts on the happy path", () => {
    renderModal();
    expect(
      screen.queryByText(/could not (create task|evaluate draft)/i),
    ).not.toBeInTheDocument();
  });

  it("renders the underlying createError message inside the modal", () => {
    renderModal({ createError: new Error("server returned 500") });
    const callout = document.querySelector(".task-create-modal-err--create");
    expect(callout).not.toBeNull();
    expect(callout).toHaveTextContent(/server returned 500/i);
  });

  it("renders the underlying evaluateError message inside the modal", () => {
    renderModal({ evaluateError: new Error("LLM timeout") });
    const callout = document.querySelector(".task-create-modal-err--evaluate");
    expect(callout).not.toBeNull();
    expect(callout).toHaveTextContent(/LLM timeout/i);
  });

  it("can render both create + evaluate errors simultaneously", () => {
    renderModal({
      createError: new Error("create boom"),
      evaluateError: new Error("eval boom"),
    });
    expect(
      document.querySelector(".task-create-modal-err--create"),
    ).toHaveTextContent(/create boom/i);
    expect(
      document.querySelector(".task-create-modal-err--evaluate"),
    ).toHaveTextContent(/eval boom/i);
  });

  it("keeps Create / Evaluate buttons reachable while an error is showing", () => {
    renderModal({
      title: "Reproduce me",
      createError: new Error("boom"),
    });
    expect(screen.getByRole("button", { name: /^evaluate$/i })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: /^create$/i })).not.toBeDisabled();
  });

  it("does not render a separate draft name field (title doubles as the draft name)", () => {
    renderModal({ draftSaveLabel: "Draft saved" });
    expect(
      screen.queryByLabelText(/^draft name$/i),
    ).not.toBeInTheDocument();
  });

  it("surfaces the draft save status near the modal heading", () => {
    renderModal({ draftSaveLabel: "Saving draft…" });
    expect(screen.getByText(/saving draft/i)).toBeInTheDocument();
  });

  it("shows Cursor model as a select with a Default option", async () => {
    const user = userEvent.setup();
    renderModal();
    await expandMoreOptions(user);
    const model = screen.getByTestId("task-create-cursor-model-select");
    expect(model.tagName).toBe("SELECT");
    expect(
      screen.getByRole("option", { name: /^default$/i }),
    ).toBeInTheDocument();
  });

  it("shows DMAP-specific fields when task type is DMAP", () => {
    renderModal({ taskType: "dmap" });
    expect(screen.getByText(/dmap configuration/i)).toBeInTheDocument();
    expect(
      screen.getByLabelText(/commits until stoppage/i),
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/dmap domain/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/direction notes/i)).toBeInTheDocument();
    expect(screen.queryByText(/done criteria/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/subtasks/i)).not.toBeInTheDocument();
  });

  it("renders the test-scenarios trigger only when onApplyTestScenario is wired", () => {
    const { unmount } = renderModal();
    expect(
      screen.queryByTestId("test-scenarios-trigger"),
    ).not.toBeInTheDocument();
    unmount();
    renderModal({ onApplyTestScenario: vi.fn() });
    expect(
      screen.getByTestId("test-scenarios-trigger"),
    ).toBeInTheDocument();
  });

  it("collapses the agent, schedule, and metadata fields behind a 'More options' disclosure by default", () => {
    renderModal();
    const toggle = screen.getByTestId("task-create-more-options-toggle");
    expect(toggle).toBeInTheDocument();
    const details = toggle.closest("details");
    expect(details).not.toBeNull();
    // Closed by default — the essentials (Title, Prompt, Done criteria,
    // Project) own the viewport on open; the advanced controls are one
    // click away.
    expect(details).not.toHaveAttribute("open");
  });

  it("toggles the 'open' attribute on the disclosure when the summary is activated", async () => {
    // Browsers hide <details> body content via the UA stylesheet when
    // [open] is absent, so flipping that attribute is the contract that
    // gates whether the agent + schedule + metadata fields are visible
    // to the operator. (JSDOM doesn't honour the UA stylesheet, so the
    // DOM-presence assertion used elsewhere wouldn't catch this.)
    const user = userEvent.setup();
    renderModal();
    const toggle = screen.getByTestId("task-create-more-options-toggle");
    const details = toggle.closest("details");
    expect(details).not.toHaveAttribute("open");
    await user.click(toggle);
    expect(details).toHaveAttribute("open");
  });

  it("summarises effective values next to the disclosure summary when fields are set", () => {
    renderModal({
      tagsCsv: "backend, api",
      milestone: "M1",
      schedule: "2026-04-19T13:00:00Z",
    });
    const toggle = screen.getByTestId("task-create-more-options-toggle");
    expect(toggle).toHaveTextContent(/Scheduled/);
    expect(toggle).toHaveTextContent(/2 tags/);
    expect(toggle).toHaveTextContent(/Milestone/);
  });

  describe("autonomy toggle", () => {
    function getAutonomyCheckbox() {
      return document.getElementById(
        "task-create-autonomy-toggle",
      ) as HTMLInputElement;
    }

    it("renders the autonomy section with the toggle checked by default", () => {
      renderModal();
      const toggle = getAutonomyCheckbox();
      expect(toggle).toBeInTheDocument();
      expect(toggle).toBeChecked();
      expect(
        screen.getByText(
          /agent will pick this task up when its scheduling and dependencies allow/i,
        ),
      ).toBeInTheDocument();
    });

    it("renders unchecked + on-hold helper copy when autonomyEnabled is false", () => {
      renderModal({ autonomyEnabled: false });
      const toggle = getAutonomyCheckbox();
      expect(toggle).not.toBeChecked();
      expect(
        screen.getByText(/will be created on hold/i),
      ).toBeInTheDocument();
    });

    it("forwards the new value through onAutonomyChange when toggled", async () => {
      const user = userEvent.setup();
      const onAutonomyChange = vi.fn();
      renderModal({ onAutonomyChange });
      await user.click(getAutonomyCheckbox());
      expect(onAutonomyChange).toHaveBeenCalledWith(false);
    });
  });

  it("opens the test-scenarios popover and forwards the picked scenario", async () => {
    const user = userEvent.setup();
    const onApplyTestScenario = vi.fn();
    renderModal({ onApplyTestScenario });
    await user.click(screen.getByTestId("test-scenarios-trigger"));
    expect(screen.getByTestId("test-scenarios-popover")).toBeInTheDocument();
    // Pick the first scenario in the catalog (any will do — we only care
    // that the modal forwards the picked object to the apply callback).
    const scenarios = await import("@/tasks/test-scenarios");
    const first = scenarios.TEST_SCENARIOS[0]!;
    await user.click(
      screen.getByTestId(`test-scenarios-pick-${first.id}`),
    );
    expect(onApplyTestScenario).toHaveBeenCalledWith(first);
    // Picking closes the popover so the operator sees the freshly-filled form.
    expect(
      screen.queryByTestId("test-scenarios-popover"),
    ).not.toBeInTheDocument();
  });
});
