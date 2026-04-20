import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import type { AppSettings, ListCursorModelsResult } from "@/api/settings";
import { settingsQueryKeys } from "@/tasks/task-query/queryKeys";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import { TaskCreateModal } from "./TaskCreateModal";

const testAppSettings: AppSettings = {
  worker_enabled: false,
  repo_root: "",
  cursor_bin: "",
  ...TASK_TEST_DEFAULTS,
  max_run_duration_seconds: 0,
  agent_pickup_delay_seconds: 5,
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

  it("shows Cursor model as a select with a Default option", () => {
    renderModal();
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
});
