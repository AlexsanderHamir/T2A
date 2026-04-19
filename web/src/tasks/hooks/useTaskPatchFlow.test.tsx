import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { useTaskPatchFlow, type TaskPatchInput } from "./useTaskPatchFlow";

vi.mock("../../api", () => ({
  patchTask: vi.fn(),
}));

import { patchTask } from "../../api";

const mockedPatch = vi.mocked(patchTask);

function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    );
  }
  return { Wrapper, queryClient, invalidateSpy };
}

const baseInput: TaskPatchInput = {
  id: "t1",
  title: "New title",
  initial_prompt: "<p>hi</p>",
  status: "ready",
  priority: "medium",
  task_type: "general",
  checklist_inherit: false,
};

describe("useTaskPatchFlow", () => {
  beforeEach(() => {
    mockedPatch.mockReset();
  });
  afterEach(() => {
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
      task_type: "general",
      checklist_inherit: false,
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
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["task-stats"] });
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
});
