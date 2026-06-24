import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { AppSettings, ListCursorModelsResult } from "@/api/settings";
import { settingsQueryKeys } from "@/tasks/task-query/queryKeys";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import {
  GIT_TEST_BRANCH_ID,
  GIT_TEST_WORKTREE_ID,
} from "@/test/handlers/git";
import { respondGlobalGitApi } from "@/test/handlers/gitGlobal";
import { APP_SETTINGS_DEFAULTS } from "@/test/settingsDefaults";
import { TaskCreateModal } from "./TaskCreateModal";

const isUiFeatureOmitted = vi.hoisted(() => vi.fn((_feature: string) => false));

vi.mock("@/launch/omittedFeatures", () => ({
  OMITTED_UI_FEATURES: {
    projects: true,
    tagsAndDependencies: true,
    schedule: true,
  },
  isUiFeatureOmitted: (feature: string) => isUiFeatureOmitted(feature),
}));

const testAppSettings: AppSettings = {
  ...APP_SETTINGS_DEFAULTS,
  ...TASK_TEST_DEFAULTS,
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
    checklistItems: [],
    onTitleChange: vi.fn(),
    onPromptChange: vi.fn(),
    onPriorityChange: vi.fn(),
    onAppendChecklistCriterion: vi.fn(),
    onUpdateChecklistRow: vi.fn(),
    onRemoveChecklistRow: vi.fn(),
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
    worktreeId: "",
    branchId: "",
    onWorktreeChange: vi.fn(),
    onBranchChange: vi.fn(),
    dependsOn: [],
    onTagsCsvChange: vi.fn(),
    onMilestoneChange: vi.fn(),
    onDependsOnChange: vi.fn(),
    appTimezone: "UTC",
    onSaveDraft: vi.fn(),
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
    <MemoryRouter>
      <QueryClientProvider client={client}>
        <TaskCreateModal {...base} {...props} />
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

/** Expand the "More options" disclosure so tests can interact with the
 *  agent + schedule + metadata controls it collapses by default. */
async function expandMoreOptions(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByTestId("task-create-more-options-toggle"));
}

describe("TaskCreateModal", () => {
  beforeEach(() => {
    isUiFeatureOmitted.mockImplementation(() => false);
  });

  it("disables Create until at least one done criterion exists", () => {
    renderModal({ checklistItems: [] });
    expect(screen.getByRole("button", { name: /^create task$/i })).toBeDisabled();
    expect(screen.getByText(/add at least one done criterion/i)).toBeInTheDocument();
  });

  it("marks Done criteria as required", () => {
    renderModal({ checklistItems: [] });
    const heading = screen.getByRole("heading", { name: /done criteria/i });
    const row = heading.closest(".task-create-checklist-title-row");
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).getByText("*")).toBeInTheDocument();
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
      screen.getByRole("button", { name: /^create task$/i }),
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
      screen.queryByText(/could not create task/i),
    ).not.toBeInTheDocument();
  });

  it("renders the underlying createError message inside the modal", () => {
    renderModal({ createError: new Error("server returned 500") });
    const callout = document.querySelector(".task-create-modal-err--create");
    expect(callout).not.toBeNull();
    expect(callout).toHaveTextContent(/server returned 500/i);
  });

  it("keeps Create button reachable while an error is showing", () => {
    renderModal({
      title: "Reproduce me",
      checklistItems: [{ text: "Ship it" }],
      worktreeId: "wt-1",
      branchId: "br-1",
      createError: new Error("boom"),
    });
    expect(screen.getByRole("button", { name: /^create task$/i })).not.toBeDisabled();
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

  it("shows Cursor model as a custom select with an Auto option", async () => {
    const user = userEvent.setup();
    renderModal();
    await expandMoreOptions(user);
    const model = screen.getByTestId("task-create-cursor-model-select");
    expect(model).toHaveAttribute("role", "combobox");
    await user.click(model);
    expect(screen.getByRole("option", { name: /^auto$/i })).toBeInTheDocument();
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

  describe("launch omissions", () => {
    it("hides schedule and tags from More options when launch flags omit them", async () => {
      isUiFeatureOmitted.mockImplementation(
        (feature) => feature === "schedule" || feature === "tagsAndDependencies",
      );
      const user = userEvent.setup();
      renderModal({
        tagsCsv: "backend, api",
        milestone: "M1",
        schedule: "2026-04-19T13:00:00Z",
      });
      const toggle = screen.getByTestId("task-create-more-options-toggle");
      expect(toggle).not.toHaveTextContent(/2 tags/);
      expect(toggle).not.toHaveTextContent(/Scheduled/);
      await expandMoreOptions(user);
      expect(screen.queryByTestId("schedule-picker-input")).not.toBeInTheDocument();
      expect(screen.queryByLabelText(/^tags$/i)).not.toBeInTheDocument();
    });
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
          /created as ready.*no other task is running/i,
        ),
      ).toBeInTheDocument();
    });

    it("renders unchecked + on-hold helper copy when autonomyEnabled is false", () => {
      renderModal({ autonomyEnabled: false });
      const toggle = getAutonomyCheckbox();
      expect(toggle).not.toBeChecked();
      expect(
        screen.getByText(/created on hold until you resume/i),
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

  describe("edit mode", () => {
    function renderEditModal(
      props?: Partial<ComponentProps<typeof TaskCreateModal>>,
    ) {
      return renderModal({
        editingTaskId: "task-123",
        editingTaskRunner: "cursor",
        composeStatus: "ready",
        onComposeStatusChange: vi.fn(),
        patchPending: false,
        patchError: null,
        formError: null,
        title: "Existing title",
        prompt: "Existing prompt",
        priority: "medium",
        checklistItems: [],
        onSubmit: vi.fn(),
        ...props,
      });
    }

    it("renders the edit dialog with the task id and current title", () => {
      renderEditModal();
      expect(
        screen.getByRole("dialog", { name: /edit task/i }),
      ).toBeInTheDocument();
      expect(screen.getByText("task-123")).toBeInTheDocument();
      expect(screen.getByDisplayValue(/existing title/i)).toBeInTheDocument();
    });

    it("calls onClose on Escape while the patch is pending (dismissibleWhileBusy)", async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();
      renderEditModal({ patchPending: true, saving: true, onClose });
      await user.keyboard("{Escape}");
      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it("still renders the busy spinner overlay while pending", () => {
      renderEditModal({ patchPending: true, saving: true });
      expect(screen.getByRole("status")).toBeInTheDocument();
    });

    it("does not render an alert region when patchError and formError are null", () => {
      renderEditModal();
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("renders the underlying patch error message when patchError is set", () => {
      renderEditModal({ patchError: "title cannot be empty" });
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(/title cannot be empty/i);
    });

    it("keeps action buttons enabled when an error is showing so the user can retry", () => {
      renderEditModal({ patchError: "boom" });
      expect(
        screen.getByRole("button", { name: /^save$/i }),
      ).not.toBeDisabled();
      expect(
        screen.getByRole("button", { name: /^cancel$/i }),
      ).not.toBeDisabled();
    });

    it("shows SchedulePicker for ready tasks inside More options", async () => {
      const user = userEvent.setup();
      renderEditModal({ composeStatus: "ready", schedule: null });
      await expandMoreOptions(user);
      expect(screen.getByTestId("schedule-picker-input")).toBeInTheDocument();
    });

    it("shows read-only pickup copy for running tasks inside More options", async () => {
      const user = userEvent.setup();
      renderEditModal({
        composeStatus: "running",
        schedule: "2026-04-22T13:00:00Z",
      });
      await expandMoreOptions(user);
      expect(
        screen.queryByTestId("schedule-picker-input"),
      ).not.toBeInTheDocument();
      expect(
        screen.getByText(
          /pickup time cannot be changed while the task is running/i,
        ),
      ).toBeInTheDocument();
    });
  });

  describe("template compose mode", () => {
    function renderTemplateModal(
      props?: Partial<ComponentProps<typeof TaskCreateModal>>,
    ) {
      return renderModal({
        composeTarget: "template",
        composeOperation: "create",
        checklistItems: [{ text: "Done when shipped" }],
        ...props,
      });
    }

    it("shows Save template as the primary action without a Save draft button", () => {
      renderTemplateModal();
      expect(
        screen.getByRole("button", { name: /^save template$/i }),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: /save draft/i }),
      ).not.toBeInTheDocument();
    });

    it("uses the New template dialog title in create mode", () => {
      renderTemplateModal();
      expect(
        screen.getByRole("dialog", { name: /new template/i }),
      ).toBeInTheDocument();
    });

    it("uses the Edit template dialog title when editing", () => {
      renderTemplateModal({ composeOperation: "edit" });
      expect(
        screen.getByRole("dialog", { name: /edit template/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /^save$/i }),
      ).toBeInTheDocument();
    });
  });

  describe("git binding", () => {
    function stubGitFetch() {
      vi.spyOn(globalThis, "fetch").mockImplementation(async (input: RequestInfo | URL) => {
        const url = typeof input === "string" ? input : input.toString();
        const git = respondGlobalGitApi(url);
        if (git) return git;
        return new Response("not found", { status: 404 });
      });
    }

    it("disables Create task until worktree and branch are selected", async () => {
      stubGitFetch();
      renderModal({ worktreeId: "", branchId: "" });
      expect(screen.getByRole("button", { name: /Create task/i })).toBeDisabled();
    });

    it("preselects when only one worktree and branch exist", async () => {
      stubGitFetch();
      const onWorktreeChange = vi.fn();
      const onBranchChange = vi.fn();
      renderModal({ onWorktreeChange, onBranchChange });
      await waitFor(() => {
        expect(onWorktreeChange).toHaveBeenCalledWith(GIT_TEST_WORKTREE_ID);
        expect(onBranchChange).toHaveBeenCalledWith(GIT_TEST_BRANCH_ID);
      });
    });
  });
});
