import { renderHook, act } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useStickyShellElevation } from "./useStickyShellElevation";

describe("useStickyShellElevation", () => {
  afterEach(() => {
    Object.defineProperty(window, "scrollY", {
      value: 0,
      writable: true,
      configurable: true,
    });
  });

  it("returns false at the top of the page", () => {
    Object.defineProperty(window, "scrollY", {
      value: 0,
      writable: true,
      configurable: true,
    });
    const { result } = renderHook(() => useStickyShellElevation());
    expect(result.current).toBe(false);
  });

  it("returns true after a scroll past the threshold", () => {
    const { result } = renderHook(() => useStickyShellElevation(4));
    expect(result.current).toBe(false);

    act(() => {
      Object.defineProperty(window, "scrollY", {
        value: 12,
        writable: true,
        configurable: true,
      });
      window.dispatchEvent(new Event("scroll"));
    });

    expect(result.current).toBe(true);
  });

  it("returns false again after scrolling back to the top", () => {
    const { result } = renderHook(() => useStickyShellElevation(4));

    act(() => {
      Object.defineProperty(window, "scrollY", {
        value: 100,
        writable: true,
        configurable: true,
      });
      window.dispatchEvent(new Event("scroll"));
    });
    expect(result.current).toBe(true);

    act(() => {
      Object.defineProperty(window, "scrollY", {
        value: 0,
        writable: true,
        configurable: true,
      });
      window.dispatchEvent(new Event("scroll"));
    });
    expect(result.current).toBe(false);
  });

  it("treats negative scrollY (over-scroll bounce) as zero", () => {
    const { result } = renderHook(() => useStickyShellElevation(4));

    act(() => {
      Object.defineProperty(window, "scrollY", {
        value: -10,
        writable: true,
        configurable: true,
      });
      window.dispatchEvent(new Event("scroll"));
    });

    expect(result.current).toBe(false);
  });

  it("respects a custom threshold", () => {
    const { result } = renderHook(() => useStickyShellElevation(50));

    act(() => {
      Object.defineProperty(window, "scrollY", {
        value: 30,
        writable: true,
        configurable: true,
      });
      window.dispatchEvent(new Event("scroll"));
    });
    expect(result.current).toBe(false);

    act(() => {
      Object.defineProperty(window, "scrollY", {
        value: 80,
        writable: true,
        configurable: true,
      });
      window.dispatchEvent(new Event("scroll"));
    });
    expect(result.current).toBe(true);
  });

  it("removes the scroll listener on unmount", () => {
    const removeSpy = vi.spyOn(window, "removeEventListener");
    const { unmount } = renderHook(() => useStickyShellElevation());
    unmount();
    expect(removeSpy).toHaveBeenCalledWith("scroll", expect.any(Function));
  });
});
