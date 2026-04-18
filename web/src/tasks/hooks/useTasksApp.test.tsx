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
  saveTaskDraft,
  getTaskDraft,
} from "../../api";

const mockedListTasks = vi.mocked(listTasks);
const mockedGetStats = vi.mocked(getTaskStats);
const mockedListDrafts = vi.mocked(listTaskDrafts);
const mockedEvaluate = vi.mocked(evaluateDraftTask);
const mockedSaveDraft = vi.mocked(saveTaskDraft);
const mockedGetDraft = vi.mocked(getTaskDraft);

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

describe("useTasksApp saveDraftMutation race", () => {
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
    mockedSaveDraft.mockReset();
    mockedGetDraft.mockReset();
  });

  it("does not stamp the autosave baseline / 'Draft saved' label onto the now-current draft when a save for a previous draft resolves late", async () => {
    // Hold the autosave for draft A so we can switch to draft B before it
    // resolves. Capture the id we sent so the resolution can echo it back -
    // that's what the server does today.
    let resolveSaveA: ((v: { id: string; name: string }) => void) | undefined;
    let savedAId: string | undefined;
    mockedSaveDraft.mockImplementationOnce((input) => {
      savedAId = input.id;
      return new Promise<{ id: string; name: string }>((resolve) => {
        resolveSaveA = resolve;
      });
    });

    // Draft B comes back from the picker with all the fields the resume
    // path stamps onto state.
    mockedGetDraft.mockResolvedValueOnce({
      id: "draft-B-id",
      name: "Draft B",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      payload: {
        title: "Draft B title",
        initial_prompt: "Draft B prompt",
        priority: "high",
        task_type: "general",
        parent_id: "",
        checklist_inherit: false,
        checklist_items: [],
        pending_subtasks: [],
      },
    });

    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTasksApp(), { wrapper: Wrapper });

    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    act(() => {
      result.current.openCreateModal();
    });
    expect(result.current.createModalOpen).toBe(true);

    // Touch a field so the autosave signature differs from the baseline -
    // otherwise saveDraftNow short-circuits before calling the API.
    act(() => {
      result.current.setNewTitle("Draft A title");
    });

    act(() => {
      result.current.saveDraftNow();
    });
    await waitFor(() => {
      expect(result.current.draftSavePending).toBe(true);
    });
    expect(savedAId).toBeDefined();
    expect(savedAId).not.toBe("draft-B-id");

    // Mid-flight: user picks draft B from the picker. resumeDraftByID
    // synchronously updates newDraftIDRef to "draft-B-id" via the
    // setNewDraftID wrapper, then stamps the form + autosave baseline with
    // draft B's data.
    await act(async () => {
      await result.current.resumeDraftByID("draft-B-id");
    });
    expect(result.current.newTitle).toBe("Draft B title");
    expect(result.current.newPrompt).toBe("Draft B prompt");
    expect(result.current.newPriority).toBe("high");

    // Now resolve draft A's save. The server echoes the id we sent, so
    // saved.id !== newDraftIDRef.current and the guard must fire.
    await act(async () => {
      resolveSaveA?.({ id: savedAId!, name: "Untitled draft" });
    });
    await waitFor(() => {
      expect(result.current.draftSavePending).toBe(false);
    });

    // The form must STILL be showing draft B - the stale resolution must
    // not have stomped any of the fields newDraftID / newTitle / etc.
    expect(result.current.newTitle).toBe("Draft B title");
    expect(result.current.newPrompt).toBe("Draft B prompt");
    expect(result.current.newPriority).toBe("high");

    // The "Draft saved" label is the user-visible proof the baseline was
    // updated. With the bug, lastDraftSavedAt was set to Date.now() and
    // the label flips to "Draft saved" - falsely claiming draft B was
    // just saved when in reality the save was for draft A. With the
    // guard, lastDraftSavedAt stays null and the label stays null.
    expect(result.current.draftSaveLabel).toBeNull();
  });

  it("updates the autosave baseline + 'Draft saved' label on the happy path (no draft switch)", async () => {
    let savedId: string | undefined;
    mockedSaveDraft.mockImplementationOnce(async (input) => {
      savedId = input.id;
      return { id: input.id!, name: input.name };
    });

    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTasksApp(), { wrapper: Wrapper });

    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    act(() => {
      result.current.openCreateModal();
    });
    act(() => {
      result.current.setNewTitle("Draft A title");
    });

    act(() => {
      result.current.saveDraftNow();
    });

    await waitFor(() => {
      expect(result.current.draftSaveLabel).toBe("Draft saved");
    });
    expect(savedId).toBeDefined();

    // Re-running saveDraftNow without changing anything must short-circuit
    // (signature now matches the baseline) - proof the baseline was actually
    // updated to the just-saved state, not skipped by the guard.
    mockedSaveDraft.mockClear();
    act(() => {
      result.current.saveDraftNow();
    });
    expect(mockedSaveDraft).not.toHaveBeenCalled();
  });

  it("baseline tracks the snapshot that was sent, not live form state, so edits made while a save is in flight still autosave on the next dispatch", async () => {
    // First save is held so we can edit the form mid-flight. The second
    // save resolves immediately so we can assert it actually fired.
    let resolveFirst: (() => void) | undefined;
    let firstInputTitle: string | undefined;
    let secondInputTitle: string | undefined;
    mockedSaveDraft.mockImplementationOnce(async (input) => {
      firstInputTitle = input.payload.title;
      await new Promise<void>((resolve) => {
        resolveFirst = resolve;
      });
      return { id: input.id!, name: input.name };
    });
    mockedSaveDraft.mockImplementationOnce(async (input) => {
      secondInputTitle = input.payload.title;
      return { id: input.id!, name: input.name };
    });

    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTasksApp(), { wrapper: Wrapper });

    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    act(() => {
      result.current.openCreateModal();
    });

    // First edit: kicks the autosave off with snapshot S1.
    act(() => {
      result.current.setNewTitle("Title v1");
    });
    act(() => {
      result.current.saveDraftNow();
    });
    await waitFor(() => {
      expect(result.current.draftSavePending).toBe(true);
    });
    expect(firstInputTitle).toBe("Title v1");

    // Mid-flight: user keeps typing. Live form signature is now S2 (the
    // "Title v2" string). With the bug, onSuccess will rebuild the
    // baseline from live form state at resolve time -> baseline = S2.
    // currentSig = S2 too, so the next saveDraftNow gate matches and
    // autosave silently skips, even though the server still has v1.
    // With the fix, onSuccess uses variables.signature = S1, so the
    // next saveDraftNow gate sees S2 != S1 and fires.
    act(() => {
      result.current.setNewTitle("Title v2");
    });

    // Resolve the first save now (after the mid-flight edit landed in
    // state). draftSavePending flips back to false.
    await act(async () => {
      resolveFirst?.();
    });
    await waitFor(() => {
      expect(result.current.draftSavePending).toBe(false);
    });

    // The user-visible damage check: the next autosave dispatch MUST send
    // "Title v2" to the server. Without the fix, the baseline matched the
    // current signature and the gate inside saveDraftNow returned early,
    // so mockedSaveDraft would not be called a second time and the v2
    // edit would be lost on the server until the next state change.
    act(() => {
      result.current.saveDraftNow();
    });
    await waitFor(() => {
      expect(mockedSaveDraft).toHaveBeenCalledTimes(2);
    });
    expect(secondInputTitle).toBe("Title v2");
  });
});
