import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { useTaskDeleteFlow } from "./useTaskDeleteFlow";

vi.mock("../../api", () => ({
  deleteTask: vi.fn(),
}));

import { deleteTask } from "../../api";

const mockedDelete = vi.mocked(deleteTask);

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

describe("useTaskDeleteFlow", () => {
  beforeEach(() => {
    mockedDelete.mockReset();
  });
  afterEach(() => {
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
});
