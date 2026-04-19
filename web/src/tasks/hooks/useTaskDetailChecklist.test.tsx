import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { FormEvent, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { taskQueryKeys } from "../task-query";
import { useTaskDetailChecklist } from "./useTaskDetailChecklist";

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
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

function newQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
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
      ({ taskId }: { taskId: string }) => useTaskDetailChecklist(taskId, qc),
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
    const { result } = renderHook(() => useTaskDetailChecklist(TASK_A, qc), {
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
    const { result } = renderHook(() => useTaskDetailChecklist(TASK_A, qc), {
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
    const { result } = renderHook(() => useTaskDetailChecklist(TASK_A, qc), {
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
    const { result } = renderHook(() => useTaskDetailChecklist(TASK_A, qc), {
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
      const { result } = renderHook(() => useTaskDetailChecklist(TASK_A, qc), {
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
      const { result } = renderHook(() => useTaskDetailChecklist(TASK_A, qc), {
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
      const { result } = renderHook(() => useTaskDetailChecklist(TASK_A, qc), {
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
      const { result } = renderHook(() => useTaskDetailChecklist(TASK_A, qc), {
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
    const { result } = renderHook(() => useTaskDetailChecklist(TASK_A, qc), {
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
});
