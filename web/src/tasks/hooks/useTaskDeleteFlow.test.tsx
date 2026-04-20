import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { useTaskDeleteFlow } from "./useTaskDeleteFlow";
import { taskQueryKeys } from "../task-query";
import { ToastProvider } from "@/shared/toast";
import { __resetOptimisticVersionsForTests, shouldSuppressSSEFor } from "./optimisticVersion";
import { settingsQueryKeys } from "../task-query";
import type { AppSettings } from "@/api/settings";
import type { Task, TaskListResponse } from "@/types";

vi.mock("../../api", () => ({
  deleteTask: vi.fn(),
}));

import { deleteTask } from "../../api";

const mockedDelete = vi.mocked(deleteTask);

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

function makeWrapper(settings: AppSettings = makeAppSettings()) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  queryClient.setQueryData(settingsQueryKeys.app(), settings);
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <ToastProvider>{children}</ToastProvider>
      </QueryClientProvider>
    );
  }
  return { Wrapper, queryClient, invalidateSpy };
}

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    id: "t1",
    title: "Some task",
    initial_prompt: "<p>do it</p>",
    status: "ready",
    priority: "low",
    task_type: "general",
    runner: "cursor",
    cursor_model: "",
    checklist_inherit: false,
    ...overrides,
  };
}

describe("useTaskDeleteFlow", () => {
  beforeEach(() => {
    mockedDelete.mockReset();
    __resetOptimisticVersionsForTests();
  });
  afterEach(() => {
    __resetOptimisticVersionsForTests();
    vi.restoreAllMocks();
  });

  it("starts with no target, no pending, no success, no error", () => {
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    expect(result.current.deleteTarget).toBeNull();
    expect(result.current.deletePending).toBe(false);
    expect(result.current.deleteSuccess).toBe(false);
    expect(result.current.deleteError).toBeNull();
    expect(result.current.deleteVariables).toBeUndefined();
  });

  it("requestDelete captures id + title and trims an empty parent_id away", () => {
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.requestDelete({ id: "t1", title: "Hello", parent_id: "  " });
    });
    expect(result.current.deleteTarget).toEqual({
      id: "t1",
      title: "Hello",
      subtaskCount: 0,
    });
  });

  it("requestDelete preserves a non-empty trimmed parent_id", () => {
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.requestDelete({ id: "c1", title: "Child", parent_id: " p1 " });
    });
    expect(result.current.deleteTarget).toEqual({
      id: "c1",
      title: "Child",
      parent_id: "p1",
      subtaskCount: 0,
    });
  });

  it("requestDelete carries a positive subtaskCount through to deleteTarget", () => {
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.requestDelete({ id: "p1", title: "Parent", subtaskCount: 3 });
    });
    expect(result.current.deleteTarget).toEqual({
      id: "p1",
      title: "Parent",
      subtaskCount: 3,
    });
  });

  it("requestDelete clamps a negative or fractional subtaskCount to a non-negative integer", () => {
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.requestDelete({ id: "x", title: "X", subtaskCount: -2 });
    });
    expect(result.current.deleteTarget?.subtaskCount).toBe(0);

    act(() => {
      result.current.requestDelete({ id: "x", title: "X", subtaskCount: 2.7 });
    });
    expect(result.current.deleteTarget?.subtaskCount).toBe(2);
  });

  it("cancelDelete clears the target without calling the API", () => {
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.requestDelete({ id: "t1", title: "X" });
    });
    act(() => {
      result.current.cancelDelete();
    });
    expect(result.current.deleteTarget).toBeNull();
    expect(mockedDelete).not.toHaveBeenCalled();
  });

  it("confirmDelete is a no-op when no target is set", () => {
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.confirmDelete();
    });
    expect(mockedDelete).not.toHaveBeenCalled();
  });

  it("confirmDelete calls the API, invalidates list+stats, clears target, fires onDeleted", async () => {
    mockedDelete.mockResolvedValueOnce(undefined as unknown as void);
    const { Wrapper, invalidateSpy } = makeWrapper();
    const onDeleted = vi.fn();
    const { result } = renderHook(() => useTaskDeleteFlow({ onDeleted }), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.requestDelete({ id: "t1", title: "X", parent_id: "p1" });
    });
    act(() => {
      result.current.confirmDelete();
    });

    await waitFor(() => {
      expect(result.current.deleteSuccess).toBe(true);
    });

    expect(mockedDelete).toHaveBeenCalledWith("t1");
    expect(result.current.deleteTarget).toBeNull();
    expect(result.current.deleteVariables).toEqual({ id: "t1", parent_id: "p1" });
    expect(onDeleted).toHaveBeenCalledWith("t1");
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["tasks", "list"],
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["task-stats"],
    });
  });

  it("surfaces API errors via deleteError without clearing the target", async () => {
    mockedDelete.mockRejectedValueOnce(new Error("nope"));
    const { Wrapper } = makeWrapper();
    const onDeleted = vi.fn();
    const { result } = renderHook(() => useTaskDeleteFlow({ onDeleted }), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.requestDelete({ id: "t1", title: "X" });
    });
    act(() => {
      result.current.confirmDelete();
    });

    await waitFor(() => {
      expect(result.current.deleteError).toBe("nope");
    });
    expect(result.current.deleteSuccess).toBe(false);
    expect(result.current.deleteTarget).toEqual({
      id: "t1",
      title: "X",
      subtaskCount: 0,
    });
    expect(onDeleted).not.toHaveBeenCalled();
  });

  it("resetError clears a settled error without firing a new request (session #34)", async () => {
    // Pins the lifecycle wiring useTasksApp uses to wipe a stale
    // deleteError when `deleteTarget` flips to null. Without this,
    // reopening any delete confirm dialog would render an old `.err`
    // callout before the user had interacted. resetError must NOT
    // call deleteTask again.
    mockedDelete.mockRejectedValueOnce(new Error("boom"));
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.requestDelete({ id: "t1", title: "X" });
    });
    act(() => {
      result.current.confirmDelete();
    });
    await waitFor(() => {
      expect(result.current.deleteError).toBe("boom");
    });
    expect(mockedDelete).toHaveBeenCalledTimes(1);
    act(() => {
      result.current.resetError();
    });
    await waitFor(() => {
      expect(result.current.deleteError).toBeNull();
    });
    expect(mockedDelete).toHaveBeenCalledTimes(1);
  });

  it("resetError is a no-op while idle (no extra reset churn)", () => {
    // Cheap idle-guard pin: useTasksApp's effect runs on every render
    // where `deleteTarget` is null (the steady-state for most of the
    // session); resetError must skip the underlying mutation.reset()
    // call when already idle so we don't churn the react-query state
    // tree on every render.
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    expect(result.current.deleteError).toBeNull();
    act(() => {
      result.current.resetError();
    });
    expect(result.current.deleteError).toBeNull();
    expect(mockedDelete).not.toHaveBeenCalled();
  });

  it("omits parent_id from the API variables when the target has no parent", async () => {
    mockedDelete.mockResolvedValueOnce(undefined as unknown as void);
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.requestDelete({ id: "root", title: "X" });
    });
    act(() => {
      result.current.confirmDelete();
    });

    await waitFor(() => {
      expect(result.current.deleteSuccess).toBe(true);
    });
    expect(result.current.deleteVariables).toEqual({ id: "root" });
    expect(result.current.deleteVariables).not.toHaveProperty("parent_id");
  });

  it("does not clobber a freshly-opened confirm dialog when a previous delete settles", async () => {
    // Hold A's delete open until we manually resolve, simulating a slow API.
    let resolveA: (() => void) | undefined;
    mockedDelete.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          resolveA = resolve;
        }) as unknown as ReturnType<typeof deleteTask>,
    );

    const { Wrapper } = makeWrapper();
    const onDeleted = vi.fn();
    const { result } = renderHook(() => useTaskDeleteFlow({ onDeleted }), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.requestDelete({ id: "A", title: "A" });
    });
    act(() => {
      result.current.confirmDelete();
    });

    await waitFor(() => {
      expect(result.current.deletePending).toBe(true);
    });

    // Mid-flight: user opens the confirm dialog for a *different* row.
    act(() => {
      result.current.requestDelete({ id: "B", title: "B" });
    });
    expect(result.current.deleteTarget).toEqual({
      id: "B",
      title: "B",
      subtaskCount: 0,
    });

    // Now A finishes successfully.
    act(() => {
      resolveA?.();
    });

    await waitFor(() => {
      expect(onDeleted).toHaveBeenCalledWith("A");
    });

    // B's confirm dialog must still be up — A's resolution must not silently
    // dismiss the unrelated second target.
    expect(result.current.deleteTarget).toEqual({
      id: "B",
      title: "B",
      subtaskCount: 0,
    });
  });

  // Optimistic delete: between click and server confirmation the row
  // is already gone from the list cache. This is the highest-impact
  // mutation for perceived speed because the round-trip can be
  // 100-300ms and "click delete -> wait -> row vanishes" feels
  // jankier than any other mutation in the app.
  it("optimistically removes the row from cached list data before the server resolves", async () => {
    let resolveFn: (() => void) | undefined;
    mockedDelete.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          resolveFn = resolve;
        }) as unknown as ReturnType<typeof deleteTask>,
    );
    const { Wrapper, queryClient } = makeWrapper();
    const list: TaskListResponse = {
      tasks: [makeTask({ id: "t1" }), makeTask({ id: "t2" })],
      limit: 50,
      offset: 0,
      has_more: false,
    };
    queryClient.setQueryData<TaskListResponse>(taskQueryKeys.list(0), list);

    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.requestDelete({ id: "t1", title: "Some task" });
    });
    act(() => {
      result.current.confirmDelete();
    });
    await waitFor(() => {
      expect(result.current.deletePending).toBe(true);
    });
    const cached = queryClient.getQueryData<TaskListResponse>(taskQueryKeys.list(0));
    expect(cached?.tasks.map((t) => t.id)).toEqual(["t2"]);
    act(() => {
      resolveFn?.();
    });
    await waitFor(() => {
      expect(result.current.deletePending).toBe(false);
    });
  });

  // Walk into nested children: deleting a subtask leaves the parent
  // cached row intact but removes the subtask from its children
  // array. Pinning prevents a regression where the visit() helper
  // forgets to recurse and only deletes top-level rows.
  it("optimistically removes a nested subtask from its parent's children array", async () => {
    let resolveFn: (() => void) | undefined;
    mockedDelete.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          resolveFn = resolve;
        }) as unknown as ReturnType<typeof deleteTask>,
    );
    const { Wrapper, queryClient } = makeWrapper();
    const child = makeTask({ id: "child", parent_id: "parent" });
    const parent = makeTask({ id: "parent" });
    parent.children = [child];
    const list: TaskListResponse = {
      tasks: [parent],
      limit: 50,
      offset: 0,
      has_more: false,
    };
    queryClient.setQueryData<TaskListResponse>(taskQueryKeys.list(0), list);

    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.requestDelete({ id: "child", title: "child", parent_id: "parent" });
    });
    act(() => {
      result.current.confirmDelete();
    });
    await waitFor(() => {
      expect(result.current.deletePending).toBe(true);
    });
    const cached = queryClient.getQueryData<TaskListResponse>(taskQueryKeys.list(0));
    expect(cached?.tasks[0]?.id).toBe("parent");
    expect(cached?.tasks[0]?.children).toEqual([]);
    act(() => {
      resolveFn?.();
    });
    await waitFor(() => {
      expect(result.current.deletePending).toBe(false);
    });
  });

  // Rollback on error: list cache restored to the pre-mutation
  // snapshot. Without this the user sees "deleted -> undeleted ->
  // deleted again on re-attempt" depending on cache state, which is
  // even more confusing than a non-optimistic delete failure.
  it("restores the list cache on server error", async () => {
    mockedDelete.mockRejectedValueOnce(new Error("perm denied"));
    const { Wrapper, queryClient } = makeWrapper();
    const list: TaskListResponse = {
      tasks: [makeTask({ id: "t1" }), makeTask({ id: "t2" })],
      limit: 50,
      offset: 0,
      has_more: false,
    };
    queryClient.setQueryData<TaskListResponse>(taskQueryKeys.list(0), list);

    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.requestDelete({ id: "t1", title: "Some task" });
    });
    act(() => {
      result.current.confirmDelete();
    });
    await waitFor(() => {
      expect(result.current.deleteError).toBe("perm denied");
    });
    const restored = queryClient.getQueryData<TaskListResponse>(taskQueryKeys.list(0));
    expect(restored?.tasks.map((t) => t.id)).toEqual(["t1", "t2"]);
  });

  // SSE-suppression contract: while a delete is in flight, the
  // optimistic-version counter for the deleted id is bumped so any
  // SSE task_updated/task_deleted echo for that task is suppressed.
  // Otherwise the echo would fire an invalidation that re-fetches
  // the list and (briefly) un-removes the row before the server
  // delete completes.
  it("bumps the optimistic-version counter so SSE echoes are suppressed in flight", async () => {
    let resolveFn: (() => void) | undefined;
    mockedDelete.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          resolveFn = resolve;
        }) as unknown as ReturnType<typeof deleteTask>,
    );
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.requestDelete({ id: "t1", title: "Some task" });
    });
    act(() => {
      result.current.confirmDelete();
    });
    await waitFor(() => {
      expect(result.current.deletePending).toBe(true);
    });
    expect(shouldSuppressSSEFor("t1")).toBe(true);
    act(() => {
      resolveFn?.();
    });
    await waitFor(() => {
      expect(result.current.deletePending).toBe(false);
    });
    expect(shouldSuppressSSEFor("t1")).toBe(false);
  });
});
