import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useTaskDeleteFlow } from "./useTaskDeleteFlow";
import { taskQueryKeys } from "../task-query";
import {
  __resetMutationGuardForTests,
  shouldSuppressTaskMutationEcho,
} from "@/tasks/sync/mutationGuard";
import type { TaskListResponse } from "@/types";
import { makeMutationTestWrapper } from "@/test/reactQuery";
import { makeTask } from "@/test/taskDefaults";

vi.mock("../../api", () => ({
  deleteTask: vi.fn(),
}));

import { deleteTask } from "../../api";

const mockedDelete = vi.mocked(deleteTask);

describe("useTaskDeleteFlow", () => {
  beforeEach(() => {
    mockedDelete.mockReset();
    __resetMutationGuardForTests();
  });
  afterEach(() => {
    __resetMutationGuardForTests();
    vi.restoreAllMocks();
  });

  it("starts with no target, no pending, no success, no error", () => {
    const { Wrapper } = makeMutationTestWrapper();
    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    expect(result.current.deleteTarget).toBeNull();
    expect(result.current.deletePending).toBe(false);
    expect(result.current.deleteSuccess).toBe(false);
    expect(result.current.deleteError).toBeNull();
    expect(result.current.deleteVariables).toBeUndefined();
  });

  it("requestDelete captures id and title", () => {
    const { Wrapper } = makeMutationTestWrapper();
    const { result } = renderHook(() => useTaskDeleteFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.requestDelete({ id: "t1", title: "Hello" });
    });
    expect(result.current.deleteTarget).toEqual({
      id: "t1",
      title: "Hello",
    });
  });

  it("cancelDelete clears the target without calling the API", () => {
    const { Wrapper } = makeMutationTestWrapper();
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
    const { Wrapper } = makeMutationTestWrapper();
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
    const { Wrapper, invalidateSpy } = makeMutationTestWrapper();
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
      expect(result.current.deleteSuccess).toBe(true);
    });

    expect(mockedDelete).toHaveBeenCalledWith("t1");
    expect(result.current.deleteTarget).toBeNull();
    expect(result.current.deleteVariables).toEqual({ id: "t1" });
    expect(onDeleted).toHaveBeenCalledWith("t1");
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["tasks", "list"],
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: taskQueryKeys.stats(),
    });
  });

  it("surfaces API errors via deleteError without clearing the target", async () => {
    mockedDelete.mockRejectedValueOnce(new Error("nope"));
    const { Wrapper } = makeMutationTestWrapper();
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
    const { Wrapper } = makeMutationTestWrapper();
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
    // tree on every render. Success is also preserved because detail-page
    // navigation reads the settled delete variables after the dialog closes.
    const { Wrapper } = makeMutationTestWrapper();
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

  it("omits parent_id from delete variables", async () => {
    mockedDelete.mockResolvedValueOnce(undefined as unknown as void);
    const { Wrapper } = makeMutationTestWrapper();
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

    const { Wrapper } = makeMutationTestWrapper();
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
    const { Wrapper, queryClient } = makeMutationTestWrapper();
    const list: TaskListResponse = {
      tasks: [makeTask({ id: "t1" }), makeTask({ id: "t2" })],
      limit: 50,
      offset: 0,
      has_more: false,
    };
    queryClient.setQueryData<TaskListResponse>(taskQueryKeys.list({ limit: 20, offset: 0 }), list);

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
    const cached = queryClient.getQueryData<TaskListResponse>(taskQueryKeys.list({ limit: 20, offset: 0 }));
    expect(cached?.tasks.map((t) => t.id)).toEqual(["t2"]);
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
    const { Wrapper, queryClient } = makeMutationTestWrapper();
    const list: TaskListResponse = {
      tasks: [makeTask({ id: "t1" }), makeTask({ id: "t2" })],
      limit: 50,
      offset: 0,
      has_more: false,
    };
    queryClient.setQueryData<TaskListResponse>(taskQueryKeys.list({ limit: 20, offset: 0 }), list);

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
    const restored = queryClient.getQueryData<TaskListResponse>(taskQueryKeys.list({ limit: 20, offset: 0 }));
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
    const { Wrapper } = makeMutationTestWrapper();
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
    expect(shouldSuppressTaskMutationEcho("t1")).toBe(true);
    act(() => {
      resolveFn?.();
    });
    await waitFor(() => {
      expect(result.current.deletePending).toBe(false);
    });
    expect(shouldSuppressTaskMutationEcho("t1")).toBe(false);
  });
});
