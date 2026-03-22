import { vi } from "vitest";

/** Minimal EventSource so components can subscribe without a real stream. */
export function stubEventSource(): void {
  vi.stubGlobal(
    "EventSource",
    class MockEventSource {
      onopen: (() => void) | null = null;
      onmessage: ((ev: MessageEvent) => void) | null = null;
      onerror: (() => void) | null = null;
      close = vi.fn();
      constructor(public url: string) {
        queueMicrotask(() => this.onopen?.());
      }
    },
  );
}
