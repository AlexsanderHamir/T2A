import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useTaskEventStream } from "./useTaskEventStream";

type MockES = {
  onopen: (() => void) | null;
  onmessage: ((ev: { data?: string }) => void) | null;
  onerror: (() => void) | null;
  close: ReturnType<typeof vi.fn>;
  readyState: number;
};

let getCurrentMockES: () => MockES | null;

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

describe("useTaskEventStream", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    class MockEventSource implements MockES {
      static latest: MockEventSource | null = null;
      onopen: (() => void) | null = null;
      onmessage: ((ev: { data?: string }) => void) | null = null;
      onerror: (() => void) | null = null;
      close = vi.fn();
      readyState = 0;
      constructor() {
        MockEventSource.latest = this;
      }
    }
    getCurrentMockES = () => MockEventSource.latest;
    vi.stubGlobal("EventSource", MockEventSource);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("debounced SSE message triggers query invalidation after delay", () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const inv = vi.spyOn(qc, "invalidateQueries");

    renderHook(() => useTaskEventStream(), {
      wrapper: createWrapper(qc),
    });

    const mockES = getCurrentMockES();
    expect(mockES).not.toBeNull();
    act(() => {
      mockES!.onopen?.();
    });
    act(() => {
      mockES!.onmessage?.({
        data: '{"type":"task_updated","id":"11111111-1111-4111-8111-111111111111"}',
      });
    });
    expect(inv).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(400);
    });
    expect(inv).toHaveBeenCalled();
  });

  it("does not invalidate queries after unmount before debounce elapses", () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const inv = vi.spyOn(qc, "invalidateQueries");

    const { unmount } = renderHook(() => useTaskEventStream(), {
      wrapper: createWrapper(qc),
    });

    const mockES = getCurrentMockES();
    expect(mockES).not.toBeNull();
    act(() => {
      mockES!.onopen?.();
    });
    act(() => {
      mockES!.onmessage?.({ data: "{}" });
    });
    act(() => {
      vi.advanceTimersByTime(100);
    });
    unmount();
    act(() => {
      vi.advanceTimersByTime(400);
    });
    expect(inv).not.toHaveBeenCalled();
  });
});
