import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { useTaskPatchFlow, type TaskPatchInput } from "./useTaskPatchFlow";
import { taskQueryKeys } from "../task-query";
import { ToastProvider } from "@/shared/toast";
import { __resetOptimisticVersionsForTests, shouldSuppressSSEFor } from "./optimisticVersion";
import { settingsQueryKeys } from "../task-query";
import type { AppSettings } from "@/api/settings";
import type { Task } from "@/types";
import { APP_SETTINGS_DEFAULTS } from "@/test/settingsDefaults";

vi.mock("../../api", () => ({
  patchTask: vi.fn(),
}));

import { patchTask } from "../../api";

const mockedPatch = vi.mocked(patchTask);

function makeAppSettings(overrides: Partial<AppSettings> = {}): AppSettings {
  return {
    ...APP_SETTINGS_DEFAULTS,
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
  // Seed the settings query so useRolloutFlags reads it synchronously
  // on first render. Without this seed the hook returns
  // {optimisticMutationsEnabled:false} for the first few renders and
  // the optimistic code path never runs in the test.
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
    title: "Original title",
    initial_prompt: "<p>orig</p>",
    status: "ready",
    priority: "low",
    runner: "cursor",
    cursor_model: "",
    ...overrides,
  };
}

const baseInput: TaskPatchInput = {
  id: "t1",
  title: "New title",
  initial_prompt: "<p>hi</p>",
  status: "ready",
  priority: "medium",
  cursor_model: "",
};

describe("useTaskPatchFlow", () => {
  beforeEach(() => {
    mockedPatch.mockReset();
    __resetOptimisticVersionsForTests();
  });
  afterEach(() => {
    __resetOptimisticVersionsForTests();
    vi.restoreAllMocks();
  });

  it("starts idle (no pending, no error)", () => {
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskPatchFlow(), {
      wrapper: Wrapper,
    });
    expect(result.current.patchPending).toBe(false);
    expect(result.current.patchError).toBeNull();
  });

  it("forwards every patch field to patchTask(id, fields) on the API call", async () => {
    mockedPatch.mockResolvedValueOnce(undefined as unknown as never);
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskPatchFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.patchTask(baseInput);
    });
    await waitFor(() => {
      expect(mockedPatch).toHaveBeenCalledTimes(1);
    });
    expect(mockedPatch).toHaveBeenCalledWith("t1", {
      title: "New title",
      initial_prompt: "<p>hi</p>",
      status: "ready",
      priority: "medium",
      cursor_model: "",
    });
  });

  it("forwards pickup_not_before when provided on the patch input", async () => {
    mockedPatch.mockResolvedValueOnce(undefined as unknown as never);
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskPatchFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.patchTask({
        ...baseInput,
        pickup_not_before: "2026-04-22T13:00:00.000Z",
      });
    });
    await waitFor(() => {
      expect(mockedPatch).toHaveBeenCalledTimes(1);
    });
    expect(mockedPatch).toHaveBeenCalledWith("t1", {
      title: "New title",
      initial_prompt: "<p>hi</p>",
      status: "ready",
      priority: "medium",
      cursor_model: "",
      pickup_not_before: "2026-04-22T13:00:00.000Z",
    });
  });

  it("flips patchPending while in flight and back to false on success", async () => {
    let resolveFn: (() => void) | undefined;
    mockedPatch.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          resolveFn = resolve;
        }) as unknown as ReturnType<typeof patchTask>,
    );
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskPatchFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.patchTask(baseInput);
    });
    await waitFor(() => {
      expect(result.current.patchPending).toBe(true);
    });
    act(() => {
      resolveFn?.();
    });
    await waitFor(() => {
      expect(result.current.patchPending).toBe(false);
    });
  });

  it("invalidates the full tasks tree + task-stats on success and fires onPatched(id)", async () => {
    mockedPatch.mockResolvedValueOnce(undefined as unknown as never);
    const { Wrapper, invalidateSpy } = makeWrapper();
    const onPatched = vi.fn();
    const { result } = renderHook(() => useTaskPatchFlow({ onPatched }), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.patchTask(baseInput);
    });
    await waitFor(() => {
      expect(onPatched).toHaveBeenCalledWith("t1");
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["tasks"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: taskQueryKeys.stats() });
  });

  it("surfaces API errors via patchError; does not call onPatched", async () => {
    mockedPatch.mockRejectedValueOnce(new Error("boom"));
    const { Wrapper } = makeWrapper();
    const onPatched = vi.fn();
    const { result } = renderHook(() => useTaskPatchFlow({ onPatched }), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.patchTask(baseInput);
    });
    await waitFor(() => {
      expect(result.current.patchError).toBe("boom");
    });
    expect(result.current.patchPending).toBe(false);
    expect(onPatched).not.toHaveBeenCalled();
  });

  it("clears patchError after a subsequent successful patch", async () => {
    mockedPatch.mockRejectedValueOnce(new Error("first-fail"));
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskPatchFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.patchTask(baseInput);
    });
    await waitFor(() => {
      expect(result.current.patchError).toBe("first-fail");
    });
    mockedPatch.mockResolvedValueOnce(undefined as unknown as never);
    act(() => {
      result.current.patchTask({ ...baseInput, id: "t2" });
    });
    await waitFor(() => {
      expect(result.current.patchError).toBeNull();
    });
  });

  it("resetError clears a settled error without firing a new request (session #34)", async () => {
    // Pins the lifecycle wiring useTasksApp uses to wipe a stale
    // patchError when `editing` flips to null. Without this, reopening
    // any edit modal would render an old `.err` callout before the
    // user had interacted. `resetError` must NOT call patchTask again.
    mockedPatch.mockRejectedValueOnce(new Error("boom"));
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskPatchFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.patchTask(baseInput);
    });
    await waitFor(() => {
      expect(result.current.patchError).toBe("boom");
    });
    expect(mockedPatch).toHaveBeenCalledTimes(1);
    act(() => {
      result.current.resetError();
    });
    await waitFor(() => {
      expect(result.current.patchError).toBeNull();
    });
    expect(mockedPatch).toHaveBeenCalledTimes(1);
  });

  it("resetError is a no-op while idle (no extra reset churn)", () => {
    // Cheap idle-guard pin: useTasksApp's effect runs on every render
    // where `editing` is null (the steady-state for most of the
    // session); resetError must skip the underlying mutation.reset()
    // call when already idle so we don't churn the react-query state
    // tree on every render.
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskPatchFlow(), {
      wrapper: Wrapper,
    });
    expect(result.current.patchError).toBeNull();
    act(() => {
      result.current.resetError();
    });
    expect(result.current.patchError).toBeNull();
    expect(mockedPatch).not.toHaveBeenCalled();
  });

  it("calls onPatched with the id from the most recent patch", async () => {
    mockedPatch.mockResolvedValue(undefined as unknown as never);
    const { Wrapper } = makeWrapper();
    const onPatched = vi.fn();
    const { result } = renderHook(() => useTaskPatchFlow({ onPatched }), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.patchTask({ ...baseInput, id: "alpha" });
    });
    await waitFor(() => {
      expect(onPatched).toHaveBeenCalledWith("alpha");
    });
    act(() => {
      result.current.patchTask({ ...baseInput, id: "beta" });
    });
    await waitFor(() => {
      expect(onPatched).toHaveBeenCalledWith("beta");
    });
    expect(onPatched).toHaveBeenCalledTimes(2);
  });

  // Optimistic apply contract: between click and server confirmation
  // the detail cache reflects the patched fields immediately. Without
  // this the user clicks "Save", waits 200ms+, then sees the change.
  // Pin: at the moment the mutation is in flight (server hasn't
  // resolved yet) getQueryData(detail) MUST already show new values.
  it("optimistically writes the patch into the detail cache before the server resolves", async () => {
    let resolveFn: (() => void) | undefined;
    mockedPatch.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          resolveFn = resolve;
        }) as unknown as ReturnType<typeof patchTask>,
    );
    const { Wrapper, queryClient } = makeWrapper();
    queryClient.setQueryData<Task>(taskQueryKeys.detail("t1"), makeTask());
    const { result } = renderHook(() => useTaskPatchFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.patchTask(baseInput);
    });
    await waitFor(() => {
      expect(result.current.patchPending).toBe(true);
    });
    const cached = queryClient.getQueryData<Task>(taskQueryKeys.detail("t1"));
    expect(cached?.title).toBe("New title");
    expect(cached?.priority).toBe("medium");
    act(() => {
      resolveFn?.();
    });
    await waitFor(() => {
      expect(result.current.patchPending).toBe(false);
    });
  });

  // Rollback contract: on server error the cache MUST snap back to
  // the pre-mutation snapshot. Without this the user sees their
  // failed edit linger as if it succeeded — exactly the false-success
  // experience optimistic UI is supposed to avoid.
  it("rolls the detail cache back to the snapshot on server error", async () => {
    mockedPatch.mockRejectedValueOnce(new Error("save failed"));
    const { Wrapper, queryClient } = makeWrapper();
    const original = makeTask();
    queryClient.setQueryData<Task>(taskQueryKeys.detail("t1"), original);
    const { result } = renderHook(() => useTaskPatchFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.patchTask(baseInput);
    });
    await waitFor(() => {
      expect(result.current.patchError).toBe("save failed");
    });
    const restored = queryClient.getQueryData<Task>(taskQueryKeys.detail("t1"));
    expect(restored).toEqual(original);
  });

  // SSE-suppression contract: while a patch is in flight the
  // optimistic-version counter is bumped so concurrent SSE echoes
  // for the same task id are suppressed (otherwise the echo would
  // race the optimistic apply and yank the row back to its
  // server-truth value mid-edit).
  it("bumps the optimistic-version counter so SSE echoes are suppressed in flight", async () => {
    let resolveFn: (() => void) | undefined;
    mockedPatch.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          resolveFn = resolve;
        }) as unknown as ReturnType<typeof patchTask>,
    );
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskPatchFlow(), {
      wrapper: Wrapper,
    });
    act(() => {
      result.current.patchTask(baseInput);
    });
    await waitFor(() => {
      expect(result.current.patchPending).toBe(true);
    });
    expect(shouldSuppressSSEFor("t1")).toBe(true);
    expect(shouldSuppressSSEFor("other-task")).toBe(false);
    act(() => {
      resolveFn?.();
    });
    await waitFor(() => {
      expect(result.current.patchPending).toBe(false);
    });
    // After settled, the version is cleared so the *next* SSE echo
    // is no longer suppressed (server truth re-converges via the
    // mutation's onSuccess invalidation).
    expect(shouldSuppressSSEFor("t1")).toBe(false);
  });
});
