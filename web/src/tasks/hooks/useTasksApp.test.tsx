import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { FormEvent, ReactNode } from "react";
import { useTasksApp } from "./useTasksApp";
import { stubEventSource } from "../../test/browserMocks";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import type { DraftTaskEvaluation, TaskDraftDetail } from "@/types";

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
  createTask,
} from "../../api";

const mockedListTasks = vi.mocked(listTasks);
const mockedGetStats = vi.mocked(getTaskStats);
const mockedListDrafts = vi.mocked(listTaskDrafts);
const mockedEvaluate = vi.mocked(evaluateDraftTask);
const mockedSaveDraft = vi.mocked(saveTaskDraft);
const mockedGetDraft = vi.mocked(getTaskDraft);
const mockedCreateTask = vi.mocked(createTask);

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
    const { result } = renderHook(() => useTasksApp({ sseLive: false }), { wrapper: Wrapper });

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
    const { result } = renderHook(() => useTasksApp({ sseLive: false }), { wrapper: Wrapper });

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
    const { result } = renderHook(() => useTasksApp({ sseLive: false }), { wrapper: Wrapper });

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
    const { result } = renderHook(() => useTasksApp({ sseLive: false }), { wrapper: Wrapper });

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
    const { result } = renderHook(() => useTasksApp({ sseLive: false }), { wrapper: Wrapper });

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

describe("useTasksApp createMutation race", () => {
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
    mockedCreateTask.mockReset();
    mockedGetDraft.mockReset();
  });

  it("does not closeCreateModal when a stale create resolves after the user switched drafts (defensive guard)", async () => {
    // Hold the create mutation so we can interleave a draft switch
    // before it resolves. Today this race is unreachable in production
    // because `Modal busy={pending}` blocks ESC/backdrop close while
    // the create is in flight, but the *hook itself* doesn't refuse a
    // programmatic resume - the moment the modal lock is loosened (or
    // an out-of-modal "submit and continue editing" path lands), the
    // unconditional `closeCreateModal()` would slam shut a draft the
    // user has since switched to. The guard documents and pins that
    // contract: the modal close is gated on the just-resolved create
    // matching the currently-active draft id.
    let resolveCreate: (() => void) | undefined;
    let createdDraftId: string | undefined;
    mockedCreateTask.mockImplementationOnce(async (input) => {
      createdDraftId = input.draft_id;
      await new Promise<void>((r) => {
        resolveCreate = r;
      });
      return {
        id: "task-1",
        title: input.title,
        initial_prompt: input.initial_prompt ?? "",
        status: "ready",
        priority: input.priority,
        runner: input.runner ?? TASK_TEST_DEFAULTS.runner,
        cursor_model: input.cursor_model ?? TASK_TEST_DEFAULTS.cursor_model,
        checklist_inherit: false,
      };
    });

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
    const { result } = renderHook(() => useTasksApp({ sseLive: false }), { wrapper: Wrapper });

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
      result.current.submitCreate({
        preventDefault: () => {},
      } as unknown as FormEvent);
    });
    await waitFor(() => {
      expect(result.current.createPending).toBe(true);
    });
    expect(createdDraftId).toBeDefined();
    expect(createdDraftId).not.toBe("draft-B-id");

    // Programmatically resume draft B mid-flight. Bypasses the UI lock
    // (which is the defensive scenario this guard exists for).
    await act(async () => {
      await result.current.resumeDraftByID("draft-B-id");
    });
    expect(result.current.newTitle).toBe("Draft B title");
    expect(result.current.createModalOpen).toBe(true);

    await act(async () => {
      resolveCreate?.();
    });
    await waitFor(() => {
      expect(result.current.createPending).toBe(false);
    });

    // The modal must STILL be open showing draft B - the stale create
    // resolution must not have closed it. Without the guard,
    // `closeCreateModal()` runs unconditionally and the modal shuts.
    expect(result.current.createModalOpen).toBe(true);
    expect(result.current.newTitle).toBe("Draft B title");
  });

  it("closeCreateModal still fires on the happy path (no draft switch)", async () => {
    mockedCreateTask.mockImplementationOnce(async (input) => ({
      id: "task-2",
      title: input.title,
      initial_prompt: input.initial_prompt ?? "",
      status: "ready",
      priority: input.priority,
      runner: input.runner ?? TASK_TEST_DEFAULTS.runner,
      cursor_model: input.cursor_model ?? TASK_TEST_DEFAULTS.cursor_model,
      checklist_inherit: false,
    }));

    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTasksApp({ sseLive: false }), { wrapper: Wrapper });

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
    expect(result.current.createModalOpen).toBe(true);

    act(() => {
      result.current.submitCreate({
        preventDefault: () => {},
      } as unknown as FormEvent);
    });

    await waitFor(() => {
      expect(result.current.createModalOpen).toBe(false);
    });
  });
});

describe("useTasksApp resumeDraftMutation race", () => {
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
    mockedGetDraft.mockReset();
  });

  function makeDraftDetail(id: string, title: string): TaskDraftDetail {
    return {
      id,
      name: `Draft ${id}`,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      payload: {
        title,
        initial_prompt: `${title} prompt`,
        priority: "medium",
        task_type: "general",
        parent_id: "",
        checklist_inherit: false,
        checklist_items: [],
        pending_subtasks: [],
      },
    } as TaskDraftDetail;
  }

  it("ignores out-of-order resume resolutions (last-clicked draft wins)", async () => {
    // Two concurrent resume requests. The user clicks B (slow GET),
    // then quickly clicks C before B resolves. We resolve C first
    // (so the form populates with C), then resolve B. Without the
    // `requestedResumeRef` guard, B's late resolution stamps the form
    // with B's payload *after* C already populated it - silently
    // reverting the user to the draft they navigated away from.
    let resolveB: ((v: TaskDraftDetail) => void) | undefined;
    let resolveC: ((v: TaskDraftDetail) => void) | undefined;
    mockedGetDraft.mockImplementation((id: string) => {
      if (id === "draft-B") {
        return new Promise((resolve) => {
          resolveB = resolve;
        });
      }
      if (id === "draft-C") {
        return new Promise((resolve) => {
          resolveC = resolve;
        });
      }
      throw new Error(`unexpected getDraft id ${id}`);
    });

    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTasksApp({ sseLive: false }), { wrapper: Wrapper });
    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    let bPromise: Promise<void> | undefined;
    let cPromise: Promise<void> | undefined;
    await act(async () => {
      bPromise = result.current.resumeDraftByID("draft-B");
      cPromise = result.current.resumeDraftByID("draft-C");
      // Yield so both `mutateAsync` calls have run their synchronous
      // prologue (registering with the mutation observer + invoking
      // the mocked `mockedGetDraft`) before we resolve either side.
      await Promise.resolve();
    });
    expect(resolveB).toBeDefined();
    expect(resolveC).toBeDefined();

    await act(async () => {
      resolveC?.(makeDraftDetail("draft-C", "Title C"));
      await cPromise;
    });
    expect(result.current.newTitle).toBe("Title C");
    expect(result.current.createModalOpen).toBe(true);

    await act(async () => {
      resolveB?.(makeDraftDetail("draft-B", "Title B"));
      await bPromise;
    });
    // The late B resolution must NOT have clobbered the form. The
    // user's most recent intent (C) is preserved.
    expect(result.current.newTitle).toBe("Title C");
  });

  it("drops a late resume resolution when the user starts a fresh draft mid-flight", async () => {
    // User clicks resume B, then changes their mind and hits "Start
    // fresh" before B resolves. `startFreshDraft` runs
    // `resetNewTaskForm`, which clears `requestedResumeRef`. When B
    // finally resolves, the post-await guard sees the ref no longer
    // matches "draft-B" and skips the form-stamp branch, preserving
    // the fresh draft the user is now editing.
    let resolveB: ((v: TaskDraftDetail) => void) | undefined;
    mockedGetDraft.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveB = resolve;
        }),
    );

    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTasksApp({ sseLive: false }), { wrapper: Wrapper });
    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    let bPromise: Promise<void> | undefined;
    act(() => {
      bPromise = result.current.resumeDraftByID("draft-B");
    });

    await act(async () => {
      await result.current.startFreshDraft();
    });
    const freshTitleBeforeResolve = result.current.newTitle;
    expect(freshTitleBeforeResolve).toBe("");
    expect(result.current.createModalOpen).toBe(true);

    await act(async () => {
      resolveB?.(makeDraftDetail("draft-B", "Title B"));
      await bPromise;
    });

    expect(result.current.newTitle).toBe("");
    expect(result.current.createModalOpen).toBe(true);
  });

  it("happy path: a normal resume populates the form", async () => {
    mockedGetDraft.mockResolvedValueOnce(makeDraftDetail("draft-A", "Title A"));

    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTasksApp({ sseLive: false }), { wrapper: Wrapper });
    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    await act(async () => {
      await result.current.resumeDraftByID("draft-A");
    });

    expect(result.current.newTitle).toBe("Title A");
    expect(result.current.createModalOpen).toBe(true);
  });
});
