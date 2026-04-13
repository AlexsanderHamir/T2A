import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { FormEvent, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
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
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

function newQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

describe("useTaskDetailSubtasks", () => {
  beforeEach(() => {
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
});
