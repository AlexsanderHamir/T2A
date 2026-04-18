import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { useTasksApp } from "./useTasksApp";
import { stubEventSource } from "../../test/browserMocks";
import type { DraftTaskEvaluation } from "@/types";

vi.mock("../../api", () => ({
  listTasks: vi.fn(),
  getTaskStats: vi.fn(),
  listTaskDrafts: vi.fn(),
  evaluateDraftTask: vi.fn(),
  getTaskDraft: vi.fn(),
  saveTaskDraft: vi.fn(),
  deleteTaskDraft: vi.fn(),
  createTask: vi.fn(),
  patchTask: vi.fn(),
  deleteTask: vi.fn(),
  addChecklistItem: vi.fn(),
}));

import {
  listTasks,
  getTaskStats,
  listTaskDrafts,
  evaluateDraftTask,
} from "../../api";

const mockedListTasks = vi.mocked(listTasks);
const mockedGetStats = vi.mocked(getTaskStats);
const mockedListDrafts = vi.mocked(listTaskDrafts);
const mockedEvaluate = vi.mocked(evaluateDraftTask);

function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    );
  }
  return { Wrapper, queryClient };
}

function makeEvaluation(overrides: Partial<DraftTaskEvaluation> = {}): DraftTaskEvaluation {
  return {
    evaluation_id: "ev-1",
    created_at: new Date().toISOString(),
    overall_score: 7,
    overall_summary: "ok",
    sections: [{ key: "clarity", score: 8 } as DraftTaskEvaluation["sections"][0]],
    cohesion_score: 7,
    cohesion_summary: "ok",
    cohesion_suggestions: [],
    ...overrides,
  } as DraftTaskEvaluation;
}

describe("useTasksApp evaluateDraftMutation race", () => {
  beforeEach(() => {
    stubEventSource();
    mockedListTasks.mockResolvedValue({
      tasks: [],
      limit: 200,
      offset: 0,
      has_more: false,
    });
    mockedGetStats.mockResolvedValue(null as unknown as never);
    mockedListDrafts.mockResolvedValue([]);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    mockedEvaluate.mockReset();
  });

  it("does not stomp latestDraftEvaluation when the user closes the modal mid-eval (stale resolution dropped)", async () => {
    let resolveEval: ((v: DraftTaskEvaluation) => void) | undefined;
    mockedEvaluate.mockImplementationOnce(
      () =>
        new Promise<DraftTaskEvaluation>((resolve) => {
          resolveEval = resolve;
        }),
    );

    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTasksApp(), { wrapper: Wrapper });

    // Wait for initial drafts query to settle so openCreateModal can take the
    // "no drafts → straight to fresh form" branch.
    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    act(() => {
      result.current.openCreateModal();
    });
    expect(result.current.createModalOpen).toBe(true);

    // Fill the minimum fields evaluateDraftBeforeCreate gates on.
    act(() => {
      result.current.setNewTitle("Draft A");
    });
    act(() => {
      result.current.setNewPriority("medium");
    });

    act(() => {
      result.current.evaluateDraftBeforeCreate();
    });
    // Eval mutation must actually be in flight; otherwise the race we are
    // trying to reproduce never happened. `mutate()` flips `isPending` on
    // the next render, so wait for it.
    await waitFor(() => {
      expect(result.current.evaluatePending).toBe(true);
    });

    // Mid-flight: user closes the modal. closeCreateModal resets the form,
    // generating a brand-new draft id and ref so the in-flight mutation
    // resolution will see a stale id.
    act(() => {
      result.current.closeCreateModal();
    });
    expect(result.current.createModalOpen).toBe(false);

    act(() => {
      resolveEval?.(makeEvaluation({ overall_summary: "for-A" }));
    });

    // Wait for the mutation to settle.
    await waitFor(() => {
      expect(result.current.evaluatePending).toBe(false);
    });

    // The stale evaluation for draft A must NOT have landed on the form
    // (which is now showing draft B's blank state).
    expect(result.current.latestDraftEvaluation).toBeNull();
  });

  it("applies the evaluation when the draft id is unchanged (happy path still works)", async () => {
    mockedEvaluate.mockResolvedValueOnce(
      makeEvaluation({ overall_summary: "applied", overall_score: 9 }),
    );

    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTasksApp(), { wrapper: Wrapper });

    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    act(() => {
      result.current.openCreateModal();
    });

    act(() => {
      result.current.setNewTitle("Draft A");
    });
    act(() => {
      result.current.setNewPriority("medium");
    });

    act(() => {
      result.current.evaluateDraftBeforeCreate();
    });

    await waitFor(() => {
      expect(result.current.latestDraftEvaluation).not.toBeNull();
    });
    expect(result.current.latestDraftEvaluation?.overallSummary).toBe(
      "applied",
    );
    expect(result.current.latestDraftEvaluation?.overallScore).toBe(9);
  });
});
