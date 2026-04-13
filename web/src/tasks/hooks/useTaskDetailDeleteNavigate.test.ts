import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { NavigateFunction } from "react-router-dom";
import { useTaskDetailDeleteNavigate } from "./useTaskDetailDeleteNavigate";

const TASK_ID = "11111111-1111-4111-8111-111111111111";
const PARENT_ID = "22222222-2222-4222-8222-222222222222";
const TASK_B = "33333333-3333-4333-8333-333333333333";

describe("useTaskDetailDeleteNavigate", () => {
  it("does not navigate without taskId", () => {
    const navigate = vi.fn();
    renderHook(() =>
      useTaskDetailDeleteNavigate("", navigate as NavigateFunction, true, {
        id: TASK_ID,
      }),
    );
    expect(navigate).not.toHaveBeenCalled();
  });

  it("does not navigate when delete is not successful", () => {
    const navigate = vi.fn();
    renderHook(() =>
      useTaskDetailDeleteNavigate(TASK_ID, navigate as NavigateFunction, false, {
        id: TASK_ID,
      }),
    );
    expect(navigate).not.toHaveBeenCalled();
  });

  it("does not navigate when variables id does not match current task", () => {
    const navigate = vi.fn();
    renderHook(() =>
      useTaskDetailDeleteNavigate(TASK_ID, navigate as NavigateFunction, true, {
        id: TASK_B,
      }),
    );
    expect(navigate).not.toHaveBeenCalled();
  });

  it("navigates to parent task when parent_id is set", async () => {
    const navigate = vi.fn();
    renderHook(() =>
      useTaskDetailDeleteNavigate(TASK_ID, navigate as NavigateFunction, true, {
        id: TASK_ID,
        parent_id: PARENT_ID,
      }),
    );
    await waitFor(() => {
      expect(navigate).toHaveBeenCalledWith(
        `/tasks/${encodeURIComponent(PARENT_ID)}`,
        { replace: true },
      );
    });
  });

  it("navigates home when there is no parent", async () => {
    const navigate = vi.fn();
    renderHook(() =>
      useTaskDetailDeleteNavigate(TASK_ID, navigate as NavigateFunction, true, {
        id: TASK_ID,
      }),
    );
    await waitFor(() => {
      expect(navigate).toHaveBeenCalledWith("/", { replace: true });
    });
  });

  it("navigates at most once until taskId changes", async () => {
    const mockNavigate = vi.fn();
    const { rerender } = renderHook(
      ({
        tid,
        ok,
        vars,
      }: {
        tid: string;
        ok: boolean;
        vars: unknown;
      }) =>
        useTaskDetailDeleteNavigate(
          tid,
          mockNavigate as NavigateFunction,
          ok,
          vars,
        ),
      {
        initialProps: {
          tid: TASK_ID,
          ok: true,
          vars: { id: TASK_ID } as unknown,
        },
      },
    );
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledTimes(1));
    rerender({ tid: TASK_ID, ok: true, vars: { id: TASK_ID } });
    expect(mockNavigate).toHaveBeenCalledTimes(1);

    mockNavigate.mockClear();
    rerender({ tid: TASK_B, ok: false, vars: undefined });
    rerender({ tid: TASK_B, ok: true, vars: { id: TASK_B } });
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledTimes(1));
  });
});
