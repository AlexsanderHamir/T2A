import { rumSSEReconnected } from "@/observability";

/** Wait this long after an error before showing disconnected (browser may reconnect). */
export const SSE_DISCONNECT_UI_MS = 900;

export type TaskEventSourceHandlers = {
  onMessage: (data: string) => void;
  onLiveChange: (live: boolean) => void;
  isActive: () => boolean;
};

/**
 * Opens GET /events and wires open/message/error handlers. Returns cleanup
 * that closes the source and clears disconnect UI timers.
 */
export function connectTaskEventSource(handlers: TaskEventSourceHandlers): () => void {
  const es = new EventSource("/events");
  let disconnectedAt: number | null = null;
  let hasOpenedOnce = false;
  let disconnectUiTimer: ReturnType<typeof setTimeout> | undefined;

  const clearDisconnectUi = () => {
    if (disconnectUiTimer !== undefined) {
      clearTimeout(disconnectUiTimer);
      disconnectUiTimer = undefined;
    }
  };

  es.onopen = () => {
    clearDisconnectUi();
    if (!handlers.isActive()) {
      return;
    }
    if (hasOpenedOnce && disconnectedAt !== null) {
      const gapMs = Math.max(0, performance.now() - disconnectedAt);
      rumSSEReconnected(gapMs);
    }
    hasOpenedOnce = true;
    disconnectedAt = null;
    handlers.onLiveChange(true);
  };

  es.onmessage = (ev) => {
    handlers.onMessage(String(ev.data ?? ""));
  };

  es.onerror = () => {
    if (disconnectedAt === null) {
      disconnectedAt = performance.now();
    }
    clearDisconnectUi();
    disconnectUiTimer = setTimeout(() => {
      disconnectUiTimer = undefined;
      if (!handlers.isActive()) {
        return;
      }
      if (es.readyState !== EventSource.OPEN) {
        handlers.onLiveChange(false);
      }
    }, SSE_DISCONNECT_UI_MS);
  };

  return () => {
    clearDisconnectUi();
    es.close();
    handlers.onLiveChange(false);
  };
}
