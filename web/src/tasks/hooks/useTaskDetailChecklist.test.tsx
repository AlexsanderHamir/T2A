import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { FormEvent, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { taskQueryKeys } from "../task-query";
import { useTaskDetailChecklist } from "./useTaskDetailChecklist";
import { ToastProvider } from "@/shared/toast";
import { settingsQueryKeys } from "../task-query";
import type { AppSettings } from "@/api";
import type { TaskChecklistResponse } from "@/types";

const { mockAdd, mockPatch, mockDelete } = vi.hoisted(() => ({
  mockAdd: vi.fn(),
  mockPatch: vi.fn(),
  mockDelete: vi.fn(),
}));

vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return {
    ...actual,
    addChecklistItem: mockAdd,
    patchChecklistItemText: mockPatch,
    deleteChecklistItem: mockDelete,
  };
});

const TASK_A = "11111111-1111-4111-8111-111111111111";
const TASK_B = "22222222-2222-4222-8222-222222222222";
const ITEM_ID = "33333333-3333-4333-8333-333333333333";

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={qc}>
        <ToastProvider>{children}</ToastProvider>
      </QueryClientProvider>
    );
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

describe("useTaskDetailChecklist", () => {
  beforeEach(() => {
    mockAdd.mockReset();
    mockPatch.mockReset();
    mockDelete.mockReset();
    mockAdd.mockResolvedValue({
      id: ITEM_ID,
      task_id: TASK_A,
      text: "criterion",
      done: false,
    });
    mockPatch.mockResolvedValue({
      id: ITEM_ID,
      task_id: TASK_A,
      text: "updated",
      done: false,
    });
    mockDelete.mockResolvedValue(undefined);
  });

  it("clears checklist modals when taskId changes", () => {
    const qc = newQueryClient();
    const { result, rerender } = renderHook(
      ({ taskId }: { taskId: string }) => useTaskDetailChecklist(taskId),
      {
        wrapper: createWrapper(qc),
        initialProps: { taskId: TASK_A },
      },
    );

    act(() => {
      result.current.openChecklistModal();
      result.current.setNewChecklistText("x");
    });
    expect(result.current.checklistModalOpen).toBe(true);

    rerender({ taskId: TASK_B });
    expect(result.current.checklistModalOpen).toBe(false);
    expect(result.current.newChecklistText).toBe("");
    expect(result.current.editCriterionModalOpen).toBe(false);
  });

  it("openChecklistModal and closeChecklistModal", () => {
    const qc = newQueryClient();
    const { result } = renderHook(() => useTaskDetailChecklist(TASK_A), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.openChecklistModal();
      result.current.setNewChecklistText("draft");
    });
    expect(result.current.checklistModalOpen).toBe(true);

    act(() => {
      result.current.closeChecklistModal();
    });
    expect(result.current.checklistModalOpen).toBe(false);
    expect(result.current.newChecklistText).toBe("");
  });

  it("openEditCriterionModal closes add modal and sets edit fields", () => {
    const qc = newQueryClient();
    const { result } = renderHook(() => useTaskDetailChecklist(TASK_A), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.openChecklistModal();
      result.current.setNewChecklistText("n");
    });
    act(() => {
      result.current.openEditCriterionModal(ITEM_ID, "old text");
    });
    expect(result.current.checklistModalOpen).toBe(false);
    expect(result.current.newChecklistText).toBe("");
    expect(result.current.editCriterionModalOpen).toBe(true);
    expect(result.current.editingChecklistItemId).toBe(ITEM_ID);
    expect(result.current.editChecklistText).toBe("old text");
  });

  it("submitNewChecklistCriterion no-ops when text is blank", () => {
    const qc = newQueryClient();
    const { result } = renderHook(() => useTaskDetailChecklist(TASK_A), {
      wrapper: createWrapper(qc),
    });

    const ev = { preventDefault: vi.fn() } as unknown as FormEvent;
    act(() => {
      result.current.setNewChecklistText("   ");
      result.current.submitNewChecklistCriterion(ev);
    });
    expect(ev.preventDefault).toHaveBeenCalled();
    expect(mockAdd).not.toHaveBeenCalled();
  });

  it("submitNewChecklistCriterion adds item, invalidates, closes add modal", async () => {
    const qc = newQueryClient();
    const inv = vi.spyOn(qc, "invalidateQueries");
    const { result } = renderHook(() => useTaskDetailChecklist(TASK_A), {
      wrapper: createWrapper(qc),
    });

    const ev = { preventDefault: vi.fn() } as unknown as FormEvent;

    act(() => {
      result.current.openChecklistModal();
      result.current.setNewChecklistText("  New  ");
    });

    await act(async () => {
      result.current.submitNewChecklistCriterion(ev);
    });

    await waitFor(() => {
      expect(mockAdd).toHaveBeenCalledWith(TASK_A, "New");
    });
    expect(inv).toHaveBeenCalled();
    await waitFor(() => {
      expect(result.current.checklistModalOpen).toBe(false);
    });
  });

  describe("addChecklistMutation race", () => {
    it("drops the form-clear + modal-close branch when the user dismissed and reopened mid-flight", async () => {
      // Race scenario: user types criterion A, submits, then (now that
      // the add ChecklistCriterionModal is dismissibleWhileBusy)
      // closes the modal mid-flight, reopens, types a different
      // criterion B. A's late onSuccess MUST NOT clear B's text or
      // close B's freshly-opened modal.
      const qc = newQueryClient();
      const inv = vi.spyOn(qc, "invalidateQueries");
      const { result } = renderHook(() => useTaskDetailChecklist(TASK_A), {
        wrapper: createWrapper(qc),
      });

      let resolveA: ((value: unknown) => void) | undefined;
      mockAdd.mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveA = resolve;
          }),
      );

      const ev = { preventDefault: vi.fn() } as unknown as FormEvent;

      act(() => {
        result.current.openChecklistModal();
        result.current.setNewChecklistText("Criterion A");
      });
      await act(async () => {
        result.current.submitNewChecklistCriterion(ev);
        await Promise.resolve();
      });
      await waitFor(() => {
        expect(result.current.addChecklistMutation.isPending).toBe(true);
      });

      act(() => {
        result.current.closeChecklistModal();
        result.current.openChecklistModal();
        result.current.setNewChecklistText("Criterion B");
      });
      expect(result.current.checklistModalOpen).toBe(true);
      expect(result.current.newChecklistText).toBe("Criterion B");

      await act(async () => {
        resolveA?.({
          id: ITEM_ID,
          task_id: TASK_A,
          text: "Criterion A",
          done: false,
        });
        await Promise.resolve();
      });

      // Server-truth invalidations DID fire — the new criterion is real.
      await waitFor(() => {
        const keys = inv.mock.calls.map((call) => call[0]?.queryKey);
        expect(keys).toEqual(
          expect.arrayContaining([
            taskQueryKeys.checklist(TASK_A),
            taskQueryKeys.detail(TASK_A),
          ]),
        );
      });
      // But the form-clear + modal-close branch was guard-dropped, so
      // Criterion B's freshly-typed text is intact.
      expect(result.current.checklistModalOpen).toBe(true);
      expect(result.current.newChecklistText).toBe("Criterion B");
    });

    it("happy path: in-flight resolution closes the add modal and clears the text", async () => {
      const qc = newQueryClient();
      const { result } = renderHook(() => useTaskDetailChecklist(TASK_A), {
        wrapper: createWrapper(qc),
      });

      const ev = { preventDefault: vi.fn() } as unknown as FormEvent;

      act(() => {
        result.current.openChecklistModal();
        result.current.setNewChecklistText("Sole");
      });

      await act(async () => {
        result.current.submitNewChecklistCriterion(ev);
      });

      await waitFor(() => {
        expect(result.current.checklistModalOpen).toBe(false);
      });
      expect(result.current.newChecklistText).toBe("");
    });
  });

  describe("updateChecklistTextMutation race", () => {
    it("drops closeEditCriterionModal() when the user reopened the edit modal on a different item mid-flight", async () => {
      const otherItemId = "44444444-4444-4444-8444-444444444444";
      const qc = newQueryClient();
      const inv = vi.spyOn(qc, "invalidateQueries");
      const { result } = renderHook(() => useTaskDetailChecklist(TASK_A), {
        wrapper: createWrapper(qc),
      });

      let resolveA: ((value: unknown) => void) | undefined;
      mockPatch.mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveA = resolve;
          }),
      );

      const ev = { preventDefault: vi.fn() } as unknown as FormEvent;

      act(() => {
        result.current.openEditCriterionModal(ITEM_ID, "old A");
        result.current.setEditChecklistText("new A");
      });
      await act(async () => {
        result.current.submitEditChecklistCriterion(ev);
        await Promise.resolve();
      });
      await waitFor(() => {
        expect(result.current.updateChecklistTextMutation.isPending).toBe(true);
      });

      act(() => {
        result.current.openEditCriterionModal(otherItemId, "old B");
      });
      expect(result.current.editingChecklistItemId).toBe(otherItemId);
      expect(result.current.editChecklistText).toBe("old B");

      await act(async () => {
        resolveA?.({
          id: ITEM_ID,
          task_id: TASK_A,
          text: "new A",
          done: false,
        });
        await Promise.resolve();
      });

      await waitFor(() => {
        const keys = inv.mock.calls.map((call) => call[0]?.queryKey);
        expect(keys).toEqual(
          expect.arrayContaining([
            taskQueryKeys.checklist(TASK_A),
            taskQueryKeys.detail(TASK_A),
          ]),
        );
      });
      expect(result.current.editCriterionModalOpen).toBe(true);
      expect(result.current.editingChecklistItemId).toBe(otherItemId);
      expect(result.current.editChecklistText).toBe("old B");
    });

    it("happy path: in-flight resolution closes the edit modal", async () => {
      const qc = newQueryClient();
      const { result } = renderHook(() => useTaskDetailChecklist(TASK_A), {
        wrapper: createWrapper(qc),
      });

      const ev = { preventDefault: vi.fn() } as unknown as FormEvent;

      act(() => {
        result.current.openEditCriterionModal(ITEM_ID, "old");
        result.current.setEditChecklistText("new");
      });
      await act(async () => {
        result.current.submitEditChecklistCriterion(ev);
      });

      await waitFor(() => {
        expect(result.current.editCriterionModalOpen).toBe(false);
      });
      expect(result.current.editingChecklistItemId).toBeNull();
    });
  });

  it("submitEditChecklistCriterion patches and closes edit modal", async () => {
    const qc = newQueryClient();
    const inv = vi.spyOn(qc, "invalidateQueries");
    const { result } = renderHook(() => useTaskDetailChecklist(TASK_A), {
      wrapper: createWrapper(qc),
    });

    const ev = { preventDefault: vi.fn() } as unknown as FormEvent;

    act(() => {
      result.current.openEditCriterionModal(ITEM_ID, "a");
      result.current.setEditChecklistText("  b  ");
    });

    await act(async () => {
      result.current.submitEditChecklistCriterion(ev);
    });

    await waitFor(() => {
      expect(mockPatch).toHaveBeenCalledWith(TASK_A, ITEM_ID, "b");
    });
    expect(inv).toHaveBeenCalled();
    await waitFor(() => {
      expect(result.current.editCriterionModalOpen).toBe(false);
    });
  });

  // Optimistic add: a synthetic checklist item appears in the cache
  // immediately on submit. Pinning this catches a regression where
  // onMutate was dropped (or the temp id collision logic broke) and
  // the user clicks "Add" -> sees nothing -> 200ms later the row
  // appears. Phase 1d explicitly calls this out.
  it("optimistically appends a synthetic checklist item on submit", async () => {
    let resolveFn: (() => void) | undefined;
    mockAdd.mockImplementationOnce(
      () => new Promise<void>((resolve) => { resolveFn = resolve; }),
    );
    const qc = newQueryClient();
    qc.setQueryData<TaskChecklistResponse>(taskQueryKeys.checklist(TASK_A), {
      items: [{ id: "i1", sort_order: 0, text: "existing", done: false }],
    });
    const { result } = renderHook(() => useTaskDetailChecklist(TASK_A), {
      wrapper: createWrapper(qc),
    });
    act(() => {
      result.current.openChecklistModal();
      result.current.setNewChecklistText("new criterion");
    });
    act(() => {
      const ev = { preventDefault: vi.fn() } as unknown as FormEvent;
      result.current.submitNewChecklistCriterion(ev);
    });
    await waitFor(() => {
      expect(result.current.addChecklistMutation.isPending).toBe(true);
    });
    const cached = qc.getQueryData<TaskChecklistResponse>(taskQueryKeys.checklist(TASK_A));
    expect(cached?.items).toHaveLength(2);
    expect(cached?.items[1]?.text).toBe("new criterion");
    expect(cached?.items[1]?.id.startsWith("optimistic-")).toBe(true);
    act(() => {
      resolveFn?.();
    });
    await waitFor(() => {
      expect(result.current.addChecklistMutation.isPending).toBe(false);
    });
  });

  // Optimistic edit: text updates in cache immediately. Without this
  // the user clicks "Save" in the edit modal, sees the old text
  // until the API resolves, then the new text snaps in.
  it("optimistically updates checklist item text on edit", async () => {
    let resolveFn: ((v: unknown) => void) | undefined;
    mockPatch.mockImplementationOnce(
      () => new Promise((resolve) => { resolveFn = resolve; }),
    );
    const qc = newQueryClient();
    qc.setQueryData<TaskChecklistResponse>(taskQueryKeys.checklist(TASK_A), {
      items: [{ id: ITEM_ID, sort_order: 0, text: "old", done: false }],
    });
    const { result } = renderHook(() => useTaskDetailChecklist(TASK_A), {
      wrapper: createWrapper(qc),
    });
    act(() => {
      result.current.openEditCriterionModal(ITEM_ID, "old");
      result.current.setEditChecklistText("new");
    });
    act(() => {
      const ev = { preventDefault: vi.fn() } as unknown as FormEvent;
      result.current.submitEditChecklistCriterion(ev);
    });
    await waitFor(() => {
      expect(result.current.updateChecklistTextMutation.isPending).toBe(true);
    });
    const cached = qc.getQueryData<TaskChecklistResponse>(taskQueryKeys.checklist(TASK_A));
    expect(cached?.items[0]?.text).toBe("new");
    act(() => {
      resolveFn?.({ items: [{ id: ITEM_ID, sort_order: 0, text: "new", done: false }] });
    });
    await waitFor(() => {
      expect(result.current.updateChecklistTextMutation.isPending).toBe(false);
    });
  });

  // Optimistic delete: row gone from cache immediately. PLUS the
  // missing detail invalidation: the original hook only invalidated
  // checklist on delete success — the parent task detail caches a
  // derived count that went stale. Plan 1d explicitly says "fix the
  // missed detail invalidation on delete while we're in there".
  it("optimistically removes checklist item AND invalidates detail on delete success", async () => {
    let resolveFn: (() => void) | undefined;
    mockDelete.mockImplementationOnce(
      () => new Promise<void>((resolve) => { resolveFn = resolve; }),
    );
    const qc = newQueryClient();
    qc.setQueryData<TaskChecklistResponse>(taskQueryKeys.checklist(TASK_A), {
      items: [
        { id: ITEM_ID, sort_order: 0, text: "doomed", done: false },
        { id: "keep", sort_order: 1, text: "keep", done: false },
      ],
    });
    const inv = vi.spyOn(qc, "invalidateQueries");
    const { result } = renderHook(() => useTaskDetailChecklist(TASK_A), {
      wrapper: createWrapper(qc),
    });
    act(() => {
      result.current.deleteChecklistMutation.mutate(ITEM_ID);
    });
    await waitFor(() => {
      expect(result.current.deleteChecklistMutation.isPending).toBe(true);
    });
    const cached = qc.getQueryData<TaskChecklistResponse>(taskQueryKeys.checklist(TASK_A));
    expect(cached?.items.map((i) => i.id)).toEqual(["keep"]);
    act(() => {
      resolveFn?.();
    });
    await waitFor(() => {
      expect(result.current.deleteChecklistMutation.isPending).toBe(false);
    });
    expect(inv).toHaveBeenCalledWith({ queryKey: taskQueryKeys.checklist(TASK_A) });
    expect(inv).toHaveBeenCalledWith({ queryKey: taskQueryKeys.detail(TASK_A) });
  });

  // Rollback on add failure: the cache snaps back to "existing only"
  // when the server rejects. Without rollback the user sees the row
  // appear, the API silently fails, and the row stays as a phantom
  // until they refresh.
  it("rolls back the optimistic add on server error", async () => {
    mockAdd.mockRejectedValueOnce(new Error("server says no"));
    const qc = newQueryClient();
    qc.setQueryData<TaskChecklistResponse>(taskQueryKeys.checklist(TASK_A), {
      items: [{ id: "i1", sort_order: 0, text: "existing", done: false }],
    });
    const { result } = renderHook(() => useTaskDetailChecklist(TASK_A), {
      wrapper: createWrapper(qc),
    });
    act(() => {
      result.current.openChecklistModal();
      result.current.setNewChecklistText("doomed");
    });
    act(() => {
      const ev = { preventDefault: vi.fn() } as unknown as FormEvent;
      result.current.submitNewChecklistCriterion(ev);
    });
    await waitFor(() => {
      expect(result.current.addChecklistMutation.isError).toBe(true);
    });
    const cached = qc.getQueryData<TaskChecklistResponse>(taskQueryKeys.checklist(TASK_A));
    expect(cached?.items.map((i) => i.id)).toEqual(["i1"]);
  });
});
