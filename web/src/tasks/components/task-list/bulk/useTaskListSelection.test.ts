import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useTaskListSelection } from "./useTaskListSelection";

describe("useTaskListSelection", () => {
  it("starts empty", () => {
    const { result } = renderHook(() =>
      useTaskListSelection(["a", "b", "c"]),
    );
    expect(result.current.selectedVisibleIds).toEqual([]);
    expect(result.current.allVisibleSelected).toBe(false);
    expect(result.current.someVisibleSelected).toBe(false);
  });

  it("toggles a single row in and out", () => {
    const { result } = renderHook(() =>
      useTaskListSelection(["a", "b", "c"]),
    );
    act(() => result.current.toggle("b"));
    expect(result.current.selectedVisibleIds).toEqual(["b"]);
    expect(result.current.someVisibleSelected).toBe(true);
    expect(result.current.allVisibleSelected).toBe(false);
    act(() => result.current.toggle("b"));
    expect(result.current.selectedVisibleIds).toEqual([]);
  });

  it("setRowSelected is idempotent and order-preserving in the visible projection", () => {
    const { result } = renderHook(() =>
      useTaskListSelection(["a", "b", "c"]),
    );
    act(() => result.current.setRowSelected("c", true));
    act(() => result.current.setRowSelected("a", true));
    act(() => result.current.setRowSelected("a", true));
    expect(result.current.selectedVisibleIds).toEqual(["a", "c"]);
    act(() => result.current.setRowSelected("b", false));
    expect(result.current.selectedVisibleIds).toEqual(["a", "c"]);
  });

  it("selectAllVisible / deselectAllVisible drive header tri-state", () => {
    const { result } = renderHook(() =>
      useTaskListSelection(["a", "b", "c"]),
    );
    act(() => result.current.selectAllVisible());
    expect(result.current.selectedVisibleIds).toEqual(["a", "b", "c"]);
    expect(result.current.allVisibleSelected).toBe(true);
    expect(result.current.someVisibleSelected).toBe(false);
    act(() => result.current.deselectAllVisible());
    expect(result.current.selectedVisibleIds).toEqual([]);
  });

  it("toggleAllVisible flips between fully-selected and empty", () => {
    const { result } = renderHook(() =>
      useTaskListSelection(["a", "b"]),
    );
    act(() => result.current.toggle("a"));
    expect(result.current.someVisibleSelected).toBe(true);
    act(() => result.current.toggleAllVisible());
    expect(result.current.allVisibleSelected).toBe(true);
    act(() => result.current.toggleAllVisible());
    expect(result.current.selectedVisibleIds).toEqual([]);
  });

  it("selectedVisibleIds projects against the *current* visible list", () => {
    const { result, rerender } = renderHook(
      ({ ids }: { ids: string[] }) => useTaskListSelection(ids),
      { initialProps: { ids: ["a", "b", "c"] } },
    );
    act(() => result.current.selectAllVisible());
    rerender({ ids: ["a", "c", "d"] });
    expect(result.current.selectedVisibleIds).toEqual(["a", "c"]);
    expect(result.current.allVisibleSelected).toBe(false);
    expect(result.current.someVisibleSelected).toBe(true);
  });

  it("clearSelection wipes everything (including off-screen ids)", () => {
    const { result, rerender } = renderHook(
      ({ ids }: { ids: string[] }) => useTaskListSelection(ids),
      { initialProps: { ids: ["a", "b"] } },
    );
    act(() => result.current.selectAllVisible());
    rerender({ ids: ["x", "y"] });
    act(() => result.current.clearSelection());
    expect(result.current.selectedIds.size).toBe(0);
  });

  it("isSelected and isVisible expose constant-time membership checks", () => {
    const { result } = renderHook(() =>
      useTaskListSelection(["a", "b"]),
    );
    act(() => result.current.toggle("a"));
    expect(result.current.isSelected("a")).toBe(true);
    expect(result.current.isSelected("b")).toBe(false);
    expect(result.current.isVisible("a")).toBe(true);
    expect(result.current.isVisible("z")).toBe(false);
  });
});
