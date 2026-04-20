import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { FormEvent, ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Task } from "@/types";
import { ToastProvider } from "@/shared/toast";
import { settingsQueryKeys, taskQueryKeys } from "../task-query";
import type { AppSettings } from "@/api";
import {
  __resetOptimisticVersionsForTests,
  shouldSuppressSSEFor,
} from "./optimisticVersion";
import { useTaskDetailSubtasks } from "./useTaskDetailSubtasks";

const { mockCreateTask, mockAddChecklistItem } = vi.hoisted(() => ({
  mockCreateTask: vi.fn(),
  mockAddChecklistItem: vi.fn(),
}));

vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return {
    ...actual,
    createTask: mockCreateTask,
    addChecklistItem: mockAddChecklistItem,
  };
});

const PARENT_ID = "11111111-1111-4111-8111-111111111111";
const OTHER_PARENT_ID = "22222222-2222-4222-8222-222222222222";
const CHILD_ID = "33333333-3333-4333-8333-333333333333";

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={qc}>
        <ToastProvider>{children}</ToastProvider>
      </QueryClientProvider>
    );
  };
}

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    id: PARENT_ID,
    title: "Parent",
    initial_prompt: "",
    status: "ready",
    priority: "medium",
    task_type: "general",
    runner: "cursor",
    cursor_model: "",
    checklist_inherit: false,
    children: [],
    ...overrides,
  };
}

function makeAppSettings(overrides: Partial<AppSettings> = {}): AppSettings {
  return {
    worker_enabled: true,
    agent_paused: false,
    runner: "cursor",
    repo_root: "",
    cursor_bin: "",
    cursor_model: "",
    max_run_duration_seconds: 0,
    agent_pickup_delay_seconds: 5,
    display_timezone: "UTC",
    optimistic_mutations_enabled: true,
    sse_replay_enabled: false,
    ...overrides,
  };
}

function newQueryClient() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  qc.setQueryData(settingsQueryKeys.app(), makeAppSettings());
  return qc;
}

describe("useTaskDetailSubtasks", () => {
  beforeEach(() => {
    __resetOptimisticVersionsForTests();
    mockCreateTask.mockReset();
    mockAddChecklistItem.mockReset();
    mockCreateTask.mockResolvedValue({
      id: CHILD_ID,
      title: "child",
      initial_prompt: "",
      status: "ready",
      priority: "medium",
      task_type: "general",
      checklist_inherit: false,
    });
    mockAddChecklistItem.mockResolvedValue({
      id: "44444444-4444-4444-8444-444444444444",
      task_id: CHILD_ID,
      text: "criterion",
      done: false,
    });
  });

  it("openSubtaskModal resets form and opens modal", () => {
    const qc = newQueryClient();
    const { result } = renderHook(() => useTaskDetailSubtasks(PARENT_ID, qc), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.setSubtaskTitle("draft");
      result.current.setSubtaskPriority("high");
      result.current.appendSubtaskChecklistCriterion("one");
    });
    expect(result.current.subtaskTitle).toBe("draft");
    expect(result.current.subtaskChecklistItems).toEqual(["one"]);

    act(() => {
      result.current.openSubtaskModal();
    });
    expect(result.current.subtaskModalOpen).toBe(true);
    expect(result.current.subtaskTitle).toBe("");
    expect(result.current.subtaskPriority).toBe("");
    expect(result.current.subtaskChecklistItems).toEqual([]);
  });

  it("closeSubtaskModal closes and clears form", () => {
    const qc = newQueryClient();
    const { result } = renderHook(() => useTaskDetailSubtasks(PARENT_ID, qc), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.openSubtaskModal();
      result.current.setSubtaskTitle("t");
      result.current.setSubtaskPriority("low");
    });
    act(() => {
      result.current.closeSubtaskModal();
    });
    expect(result.current.subtaskModalOpen).toBe(false);
    expect(result.current.subtaskTitle).toBe("");
    expect(result.current.subtaskPriority).toBe("");
  });

  it("resets subtask UI when taskId changes", () => {
    const qc = newQueryClient();
    const { result, rerender } = renderHook(
      ({ taskId }: { taskId: string }) => useTaskDetailSubtasks(taskId, qc),
      {
        wrapper: createWrapper(qc),
        initialProps: { taskId: PARENT_ID },
      },
    );

    act(() => {
      result.current.openSubtaskModal();
      result.current.setSubtaskTitle("keep");
    });
    expect(result.current.subtaskModalOpen).toBe(true);

    rerender({ taskId: OTHER_PARENT_ID });
    expect(result.current.subtaskModalOpen).toBe(false);
    expect(result.current.subtaskTitle).toBe("");
  });

  it("clears checklist rows when checklist inherit is enabled", () => {
    const qc = newQueryClient();
    const { result } = renderHook(() => useTaskDetailSubtasks(PARENT_ID, qc), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.appendSubtaskChecklistCriterion("a");
      result.current.appendSubtaskChecklistCriterion("b");
    });
    expect(result.current.subtaskChecklistItems).toEqual(["a", "b"]);

    act(() => {
      result.current.setSubtaskInherit(true);
    });
    expect(result.current.subtaskChecklistItems).toEqual([]);
  });

  it("append trims; remove and update rows", () => {
    const qc = newQueryClient();
    const { result } = renderHook(() => useTaskDetailSubtasks(PARENT_ID, qc), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.appendSubtaskChecklistCriterion("  ");
      result.current.appendSubtaskChecklistCriterion("  x  ");
    });
    expect(result.current.subtaskChecklistItems).toEqual(["x"]);

    act(() => {
      result.current.appendSubtaskChecklistCriterion("y");
    });
    act(() => {
      result.current.removeSubtaskChecklistRow(0);
    });
    expect(result.current.subtaskChecklistItems).toEqual(["y"]);

    act(() => {
      result.current.updateSubtaskChecklistRow(0, "  z  ");
    });
    expect(result.current.subtaskChecklistItems).toEqual(["z"]);

    act(() => {
      result.current.updateSubtaskChecklistRow(0, "   ");
    });
    expect(result.current.subtaskChecklistItems).toEqual(["z"]);
  });

  it("submitNewSubtask no-ops without title or priority", () => {
    const qc = newQueryClient();
    const { result } = renderHook(() => useTaskDetailSubtasks(PARENT_ID, qc), {
      wrapper: createWrapper(qc),
    });

    const ev = { preventDefault: vi.fn() } as unknown as FormEvent;

    act(() => {
      result.current.setSubtaskTitle("   ");
      result.current.setSubtaskPriority("");
      result.current.submitNewSubtask(ev);
    });
    expect(ev.preventDefault).toHaveBeenCalled();
    expect(mockCreateTask).not.toHaveBeenCalled();
  });

  it("submitNewSubtask creates child, optional checklist items, invalidates, closes modal", async () => {
    const qc = newQueryClient();
    const inv = vi.spyOn(qc, "invalidateQueries");
    const { result } = renderHook(() => useTaskDetailSubtasks(PARENT_ID, qc), {
      wrapper: createWrapper(qc),
    });

    const ev = { preventDefault: vi.fn() } as unknown as FormEvent;

    act(() => {
      result.current.setSubtaskTitle("Sub");
      result.current.setSubtaskPrompt("Do it");
      result.current.setSubtaskPriority("high");
      result.current.setSubtaskTaskType("feature");
      result.current.appendSubtaskChecklistCriterion("c1");
      result.current.appendSubtaskChecklistCriterion("c2");
    });

    await act(async () => {
      result.current.submitNewSubtask(ev);
    });

    await waitFor(() => {
      expect(mockCreateTask).toHaveBeenCalledWith(
        expect.objectContaining({
          title: "Sub",
          initial_prompt: "Do it",
          priority: "high",
          task_type: "feature",
          parent_id: PARENT_ID,
          checklist_inherit: false,
        }),
      );
    });

    expect(mockAddChecklistItem).toHaveBeenCalledWith(CHILD_ID, "c1");
    expect(mockAddChecklistItem).toHaveBeenCalledWith(CHILD_ID, "c2");
    expect(inv).toHaveBeenCalled();
    await waitFor(() => {
      expect(result.current.subtaskModalOpen).toBe(false);
    });
  });

  describe("createSubtaskMutation race", () => {
    it("drops the form-clear + modal-close branch when the user dismissed and reopened mid-flight", async () => {
      // Race scenario: user fills + submits subtask A, then (now that
      // SubtaskCreateModal is dismissibleWhileBusy) closes the modal
      // mid-flight, reopens it, types a different subtask B. A's
      // late `onSuccess` MUST NOT closeSubtaskModal()/resetSubtaskForm()
      // — that would slam shut B's form and erase what the user typed.
      const qc = newQueryClient();
      const inv = vi.spyOn(qc, "invalidateQueries");
      const { result } = renderHook(() => useTaskDetailSubtasks(PARENT_ID, qc), {
        wrapper: createWrapper(qc),
      });

      let resolveA: ((value: unknown) => void) | undefined;
      mockCreateTask.mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveA = resolve;
          }),
      );

      const ev = { preventDefault: vi.fn() } as unknown as FormEvent;

      act(() => {
        result.current.openSubtaskModal();
        result.current.setSubtaskTitle("Subtask A");
        result.current.setSubtaskPriority("high");
      });
      await act(async () => {
        result.current.submitNewSubtask(ev);
        await Promise.resolve();
      });
      await waitFor(() => {
        expect(result.current.createSubtaskMutation.isPending).toBe(true);
      });

      act(() => {
        result.current.closeSubtaskModal();
        result.current.openSubtaskModal();
        result.current.setSubtaskTitle("Subtask B");
        result.current.setSubtaskPriority("low");
      });
      expect(result.current.subtaskModalOpen).toBe(true);
      expect(result.current.subtaskTitle).toBe("Subtask B");

      await act(async () => {
        resolveA?.({
          id: CHILD_ID,
          title: "Subtask A",
          initial_prompt: "",
          status: "ready",
          priority: "high",
          task_type: "general",
          checklist_inherit: false,
        });
        await Promise.resolve();
      });

      // Server-truth invalidations DID fire — the subtask is real.
      await waitFor(() => {
        const keys = inv.mock.calls.map((call) => call[0]?.queryKey);
        expect(keys).toEqual(
          expect.arrayContaining([
            taskQueryKeys.detail(PARENT_ID),
            taskQueryKeys.listRoot(),
          ]),
        );
      });
      // But the form-clear + modal-close branch was guarded out so
      // Subtask B's freshly-typed form is intact.
      expect(result.current.subtaskModalOpen).toBe(true);
      expect(result.current.subtaskTitle).toBe("Subtask B");
      expect(result.current.subtaskPriority).toBe("low");
    });

    it("happy path: in-flight resolution closes the modal and resets the form", async () => {
      const qc = newQueryClient();
      const { result } = renderHook(() => useTaskDetailSubtasks(PARENT_ID, qc), {
        wrapper: createWrapper(qc),
      });

      const ev = { preventDefault: vi.fn() } as unknown as FormEvent;

      act(() => {
        result.current.openSubtaskModal();
        result.current.setSubtaskTitle("Sub");
        result.current.setSubtaskPriority("high");
      });

      await act(async () => {
        result.current.submitNewSubtask(ev);
      });

      await waitFor(() => {
        expect(result.current.subtaskModalOpen).toBe(false);
      });
      expect(result.current.subtaskTitle).toBe("");
      expect(result.current.subtaskPriority).toBe("");
    });
  });

  it("submitNewSubtask skips addChecklistItem when inheriting checklist", async () => {
    const qc = newQueryClient();
    const { result } = renderHook(() => useTaskDetailSubtasks(PARENT_ID, qc), {
      wrapper: createWrapper(qc),
    });

    const ev = { preventDefault: vi.fn() } as unknown as FormEvent;

    act(() => {
      result.current.setSubtaskTitle("Sub");
      result.current.setSubtaskPriority("medium");
      result.current.appendSubtaskChecklistCriterion("ignored");
      result.current.setSubtaskInherit(true);
    });

    mockCreateTask.mockResolvedValueOnce({
      id: CHILD_ID,
      title: "Sub",
      initial_prompt: "",
      status: "ready",
      priority: "medium",
      task_type: "general",
      checklist_inherit: true,
    });

    await act(async () => {
      result.current.submitNewSubtask(ev);
    });

    await waitFor(() => expect(mockCreateTask).toHaveBeenCalled());
    expect(mockAddChecklistItem).not.toHaveBeenCalled();
  });

  describe("optimistic subtask create", () => {
    afterEach(() => {
      __resetOptimisticVersionsForTests();
    });

    it("inserts a synthetic subtask into the parent's children immediately", async () => {
      const qc = newQueryClient();
      qc.setQueryData(taskQueryKeys.detail(PARENT_ID), makeTask());

      let resolveCreate: ((value: Task) => void) | undefined;
      mockCreateTask.mockImplementationOnce(
        () =>
          new Promise<Task>((resolve) => {
            resolveCreate = resolve;
          }),
      );

      const { result } = renderHook(() => useTaskDetailSubtasks(PARENT_ID, qc), {
        wrapper: createWrapper(qc),
      });

      const ev = { preventDefault: vi.fn() } as unknown as FormEvent;
      act(() => {
        result.current.openSubtaskModal();
        result.current.setSubtaskTitle("Optimistic child");
        result.current.setSubtaskPriority("high");
      });
      await act(async () => {
        result.current.submitNewSubtask(ev);
        await Promise.resolve();
      });

      const cached = qc.getQueryData<Task>(taskQueryKeys.detail(PARENT_ID));
      expect(cached?.children).toHaveLength(1);
      expect(cached?.children?.[0]?.title).toBe("Optimistic child");
      expect(cached?.children?.[0]?.id).toMatch(/^optimistic-subtask-/);
      expect(shouldSuppressSSEFor(PARENT_ID)).toBe(true);

      await act(async () => {
        resolveCreate?.({
          id: CHILD_ID,
          title: "Optimistic child",
          initial_prompt: "",
          status: "ready",
          priority: "high",
          task_type: "general",
          runner: "cursor",
          cursor_model: "",
          checklist_inherit: false,
        } as Task);
        await Promise.resolve();
      });
      await waitFor(() => {
        expect(result.current.createSubtaskMutation.isSuccess).toBe(true);
      });
      expect(shouldSuppressSSEFor(PARENT_ID)).toBe(false);
    });

    it("rolls back the synthetic subtask when the server rejects", async () => {
      const qc = newQueryClient();
      qc.setQueryData(taskQueryKeys.detail(PARENT_ID), makeTask());

      mockCreateTask.mockRejectedValueOnce(new Error("boom"));

      const { result } = renderHook(() => useTaskDetailSubtasks(PARENT_ID, qc), {
        wrapper: createWrapper(qc),
      });

      const ev = { preventDefault: vi.fn() } as unknown as FormEvent;
      act(() => {
        result.current.openSubtaskModal();
        result.current.setSubtaskTitle("Doomed");
        result.current.setSubtaskPriority("low");
      });

      await act(async () => {
        result.current.submitNewSubtask(ev);
      });
      await waitFor(() => {
        expect(result.current.createSubtaskMutation.isError).toBe(true);
      });

      const cached = qc.getQueryData<Task>(taskQueryKeys.detail(PARENT_ID));
      expect(cached?.children ?? []).toHaveLength(0);
      expect(shouldSuppressSSEFor(PARENT_ID)).toBe(false);
    });
  });
});
