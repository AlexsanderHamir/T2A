import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { FormEvent, ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Task, TaskDraftDetail } from "@/types";
import { DEFAULT_PROJECT_ID } from "@/types";
import { settingsQueryKeys } from "../../task-query";
import { useTaskCreateFlow } from "./useTaskCreateFlow";

vi.mock("@/api", () => ({
  createTask: vi.fn(),
  deleteTaskDraft: vi.fn(),
  evaluateDraftTask: vi.fn(),
  getTaskDraft: vi.fn(),
  listTaskDrafts: vi.fn(),
  patchTask: vi.fn(),
  saveTaskDraft: vi.fn(),
}));

import {
  createTask,
  getTaskDraft,
  listTaskDrafts,
  saveTaskDraft,
} from "@/api";

const mockedCreateTask = vi.mocked(createTask);
const mockedListDrafts = vi.mocked(listTaskDrafts);
const mockedSaveDraft = vi.mocked(saveTaskDraft);
const mockedGetDraft = vi.mocked(getTaskDraft);

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    id: "task-1",
    title: "Fresh task",
    initial_prompt: "",
    status: "ready",
    priority: "medium",
    runner: "cursor",
    cursor_model: "",
    project_id: DEFAULT_PROJECT_ID,
    ...overrides,
  };
}

function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  queryClient.setQueryData(settingsQueryKeys.app(), {
    agent_paused: false,
    runner: "cursor",
    repo_root: "",
    cursor_bin: "",
    cursor_model: "",
    max_run_duration_seconds: 0,
    stream_idle_stuck_seconds: 60,
    agent_pickup_delay_seconds: 5,
    display_timezone: "UTC",
    optimistic_mutations_enabled: false,
    sse_replay_enabled: false,
  });

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }

  return { Wrapper };
}

describe("useTaskCreateFlow", () => {
  beforeEach(() => {
    mockedCreateTask.mockResolvedValue(makeTask());
    mockedListDrafts.mockResolvedValue([]);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("submits the default project for a fresh task when the dropdown is left unchanged", async () => {
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskCreateFlow(), { wrapper: Wrapper });

    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    act(() => {
      result.current.openCreateModal();
      result.current.setNewTitle("Fresh task");
      result.current.setNewPriority("medium");
      result.current.appendNewChecklistCriterion("Ship with tests");
    });

    act(() => {
      result.current.submitCreate({
        preventDefault: vi.fn(),
      } as unknown as FormEvent);
    });

    await waitFor(() => {
      expect(mockedCreateTask).toHaveBeenCalledTimes(1);
    });
    expect(mockedCreateTask).toHaveBeenCalledWith(
      expect.objectContaining({
        project_id: DEFAULT_PROJECT_ID,
        checklist_items: [{ text: "Ship with tests" }],
      }),
    );
  });

  it("does not call createTask when done criteria are empty", async () => {
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskCreateFlow(), { wrapper: Wrapper });

    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    act(() => {
      result.current.openCreateModal();
      result.current.setNewTitle("No criteria");
      result.current.setNewPriority("medium");
    });

    act(() => {
      result.current.submitCreate({
        preventDefault: vi.fn(),
      } as unknown as FormEvent);
    });

    expect(mockedCreateTask).not.toHaveBeenCalled();
    expect(result.current.createFormError).toMatch(/done criterion/i);
  });

  it("forwards the operator's project_context_item_ids on create", async () => {
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskCreateFlow(), {
      wrapper: Wrapper,
    });

    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    act(() => {
      result.current.openCreateModal();
      result.current.setNewTitle("With context");
      result.current.setNewPriority("medium");
      result.current.setNewProjectID("project-7");
      result.current.setNewProjectContextItemIDs(["ctx-a", "ctx-b"]);
      result.current.appendNewChecklistCriterion("Attach context docs");
    });

    act(() => {
      result.current.submitCreate({
        preventDefault: vi.fn(),
      } as unknown as FormEvent);
    });

    await waitFor(() => {
      expect(mockedCreateTask).toHaveBeenCalledTimes(1);
    });
    expect(mockedCreateTask).toHaveBeenCalledWith(
      expect.objectContaining({
        project_id: "project-7",
        project_context_item_ids: ["ctx-a", "ctx-b"],
      }),
    );
  });

  it("persists project + context selections in the autosaved draft payload", async () => {
    mockedSaveDraft.mockResolvedValue({ id: "draft-saved", name: "Untitled" });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskCreateFlow(), {
      wrapper: Wrapper,
    });

    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    act(() => {
      result.current.openCreateModal();
      result.current.setNewTitle("Persisted");
      result.current.setNewPriority("medium");
      result.current.setNewProjectID("project-9");
      result.current.setNewProjectContextItemIDs(["ctx-3"]);
    });

    await act(async () => {
      await result.current.saveDraftNow();
    });

    expect(mockedSaveDraft).toHaveBeenCalled();
    const last = mockedSaveDraft.mock.calls.at(-1)?.[0];
    expect(last?.payload.project_id).toBe("project-9");
    expect(last?.payload.project_context_item_ids).toEqual(["ctx-3"]);
  });

  it("forwards automation_selections on create and draft autosave", async () => {
    mockedSaveDraft.mockResolvedValue({ id: "draft-saved", name: "Untitled" });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskCreateFlow(), {
      wrapper: Wrapper,
    });

    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    act(() => {
      result.current.openCreateModal();
      result.current.setNewTitle("With automations");
      result.current.setNewPriority("medium");
      result.current.setNewAutomationSelections([
        { automation_id: "auto-1", state: "yes" },
        { automation_id: "auto-2", state: "no" },
      ]);
      result.current.appendNewChecklistCriterion("Ship it");
    });

    await act(async () => {
      await result.current.saveDraftNow();
    });

    const draftPayload = mockedSaveDraft.mock.calls.at(-1)?.[0]?.payload;
    expect(draftPayload?.automation_selections).toEqual([
      { automation_id: "auto-1", state: "yes" },
      { automation_id: "auto-2", state: "no" },
    ]);

    act(() => {
      result.current.submitCreate({
        preventDefault: vi.fn(),
      } as unknown as FormEvent);
    });

    await waitFor(() => {
      expect(mockedCreateTask).toHaveBeenCalledTimes(1);
    });
    expect(mockedCreateTask).toHaveBeenCalledWith(
      expect.objectContaining({
        automation_selections: [
          { automation_id: "auto-1", state: "yes" },
          { automation_id: "auto-2", state: "no" },
        ],
      }),
    );
  });

  it("applyTestScenario fills title / prompt / priority / criteria with zero typing", async () => {
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskCreateFlow(), {
      wrapper: Wrapper,
    });

    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    act(() => {
      result.current.openCreateModal();
    });

    const scenarios = await import("../../test-scenarios");
    const scenario = scenarios.TEST_SCENARIOS.find(
      (s) => s.criteria.length > 0,
    );
    expect(scenario).toBeDefined();
    if (!scenario) throw new Error("expected at least one scenario");

    act(() => {
      result.current.applyTestScenario(scenario);
    });

    expect(result.current.newTitle).toBe(scenario.title);
    expect(result.current.newPriority).toBe(scenario.priority);
    expect(result.current.newChecklistItems).toEqual(
      scenario.criteria.map((item) => ({
        text: item.text,
        ...(item.verify_commands?.length
          ? { verify_commands: item.verify_commands }
          : {}),
      })),
    );
    const firstLine = scenario.prompt.split("\n", 1)[0]!;
    expect(result.current.newPrompt).toContain(firstLine);
  });

  it("restores project + context selections when resuming a draft", async () => {
    const draft: TaskDraftDetail = {
      id: "draft-resume",
      name: "Untitled",
      created_at: "2026-04-29T00:00:00Z",
      updated_at: "2026-04-29T00:00:00Z",
      payload: {
        title: "Resumed",
        initial_prompt: "<p>Body</p>",
        priority: "high",
        runner: "cursor",
        cursor_model: "",
        project_id: "project-resume",
        project_context_item_ids: ["ctx-r1", "ctx-r2"],
        checklist_items: [],
      },
    };
    mockedGetDraft.mockResolvedValue(draft);

    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskCreateFlow(), {
      wrapper: Wrapper,
    });

    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    await act(async () => {
      await result.current.resumeDraftByID("draft-resume");
    });

    expect(result.current.newProjectID).toBe("project-resume");
    expect(result.current.newProjectContextItemIDs).toEqual([
      "ctx-r1",
      "ctx-r2",
    ]);
  });

  it("restores verify commands when resuming a draft", async () => {
    const draft: TaskDraftDetail = {
      id: "draft-verify-cmds",
      name: "Untitled",
      created_at: "2026-04-29T00:00:00Z",
      updated_at: "2026-04-29T00:00:00Z",
      payload: {
        title: "Resumed",
        initial_prompt: "<p>Body</p>",
        priority: "high",
        runner: "cursor",
        cursor_model: "",
        checklist_items: [
          {
            text: "Ship with tests",
            verify_commands: [
              { command: "go test ./...", expected_outcome: "exit 0" },
            ],
          },
        ],
      },
    };
    mockedGetDraft.mockResolvedValue(draft);

    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskCreateFlow(), {
      wrapper: Wrapper,
    });

    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    await act(async () => {
      await result.current.resumeDraftByID("draft-verify-cmds");
    });

    expect(result.current.newChecklistItems).toEqual([
      {
        text: "Ship with tests",
        verify_commands: [
          { command: "go test ./...", expected_outcome: "exit 0" },
        ],
      },
    ]);
  });

  it("sets createModalAssignmentLocked when opening with lockProjectAssignment", async () => {
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskCreateFlow(), {
      wrapper: Wrapper,
    });

    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    act(() => {
      result.current.openCreateModal({
        projectID: "project-locked",
        lockProjectAssignment: true,
      });
    });

    expect(result.current.createModalOpen).toBe(true);
    expect(result.current.newProjectID).toBe("project-locked");
    expect(result.current.createModalAssignmentLocked).toBe(true);
  });
});
