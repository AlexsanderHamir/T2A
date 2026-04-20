import {
  createContext,
  memo,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type PropsWithChildren,
  type ReactNode,
} from "react";
import "./toast.css";

/**
 * Lightweight toast surface. Used by mutation hooks (Phase 1) to
 * surface optimistic-rollback notifications when a mutation fails:
 * the cache is reverted silently and the user is told *why* the row
 * snapped back. Without a toast layer the rollback would feel like
 * a UI bug ("I clicked, it changed, then it un-changed??").
 *
 * Design pins:
 *  - Stack capped at MAX_TOASTS so a runaway error loop can't fill
 *    the viewport.
 *  - Auto-dismiss after AUTO_DISMISS_MS so the user doesn't have to
 *    chase down ephemeral status with the keyboard.
 *  - Dedupe by `(kind, message)` within DEDUPE_WINDOW_MS so a
 *    systemic failure (e.g. server returning 500 to every PATCH)
 *    surfaces ONE toast, not 12.
 *  - role="status" + aria-live="polite" so screen readers announce
 *    rollbacks without yanking focus.
 *  - Reduced-motion handled at the CSS layer (toast.css) — the
 *    animation keyframe is dropped under prefers-reduced-motion.
 */

export type ToastKind = "error" | "success" | "info";

export interface ToastItem {
  id: number;
  kind: ToastKind;
  message: string;
  expiresAt: number;
}

interface ToastContextShape {
  push: (kind: ToastKind, message: string) => void;
  error: (message: string) => void;
  success: (message: string) => void;
  info: (message: string) => void;
  dismiss: (id: number) => void;
  toasts: readonly ToastItem[];
}

const ToastContext = createContext<ToastContextShape | null>(null);

const MAX_TOASTS = 3;
const AUTO_DISMISS_MS = 4_000;
const DEDUPE_WINDOW_MS = 5_000;

let toastIdCounter = 0;

function nextToastId(): number {
  toastIdCounter += 1;
  return toastIdCounter;
}

interface DedupeEntry {
  message: string;
  kind: ToastKind;
  at: number;
}

export function ToastProvider({ children }: PropsWithChildren): ReactNode {
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const dedupeRef = useRef<DedupeEntry[]>([]);
  const timersRef = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map());

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
    const timer = timersRef.current.get(id);
    if (timer) {
      clearTimeout(timer);
      timersRef.current.delete(id);
    }
  }, []);

  const push = useCallback(
    (kind: ToastKind, message: string) => {
      const trimmed = message.trim();
      if (!trimmed) return;
      const now = Date.now();
      // Dedupe: drop entries past the window, then check for a live
      // duplicate. The list is short (<=3 active) so an O(n) scan
      // is fine and avoids dragging in a heavier data structure.
      dedupeRef.current = dedupeRef.current.filter((e) => now - e.at < DEDUPE_WINDOW_MS);
      const isDuplicate = dedupeRef.current.some(
        (e) => e.kind === kind && e.message === trimmed,
      );
      if (isDuplicate) return;
      dedupeRef.current.push({ kind, message: trimmed, at: now });

      const id = nextToastId();
      const expiresAt = now + AUTO_DISMISS_MS;
      const next: ToastItem = { id, kind, message: trimmed, expiresAt };
      setToasts((prev) => {
        const merged = [...prev, next];
        // Cap stack length: drop OLDEST so the most recent rollback
        // is always visible (the user just clicked it, they want
        // feedback on the action they just took).
        if (merged.length > MAX_TOASTS) {
          const trimmedList = merged.slice(merged.length - MAX_TOASTS);
          // Cancel timers for the entries we dropped to avoid a
          // setTimeout firing against state that no longer holds them.
          for (const dropped of merged.slice(0, merged.length - MAX_TOASTS)) {
            const t = timersRef.current.get(dropped.id);
            if (t) {
              clearTimeout(t);
              timersRef.current.delete(dropped.id);
            }
          }
          return trimmedList;
        }
        return merged;
      });
      const timer = setTimeout(() => dismiss(id), AUTO_DISMISS_MS);
      timersRef.current.set(id, timer);
    },
    [dismiss],
  );

  const error = useCallback((message: string) => push("error", message), [push]);
  const success = useCallback((message: string) => push("success", message), [push]);
  const info = useCallback((message: string) => push("info", message), [push]);

  useEffect(() => {
    const timers = timersRef.current;
    return () => {
      for (const t of timers.values()) {
        clearTimeout(t);
      }
      timers.clear();
    };
  }, []);

  const value = useMemo<ToastContextShape>(
    () => ({ push, error, success, info, dismiss, toasts }),
    [push, error, success, info, dismiss, toasts],
  );

  return (
    <ToastContext.Provider value={value}>
      {children}
      <ToastStack toasts={toasts} onDismiss={dismiss} />
    </ToastContext.Provider>
  );
}

interface ToastStackProps {
  toasts: readonly ToastItem[];
  onDismiss: (id: number) => void;
}

const ToastStack = memo(function ToastStack({ toasts, onDismiss }: ToastStackProps): ReactNode {
  if (toasts.length === 0) return null;
  return (
    <div className="toast-stack" aria-live="polite" aria-atomic="false">
      {toasts.map((t) => (
        <div
          key={t.id}
          role="status"
          className={`toast toast--${t.kind}`}
          data-testid={`toast-${t.kind}`}
        >
          <div className="toast__message">{t.message}</div>
          <button
            type="button"
            className="toast__close"
            aria-label="Dismiss notification"
            onClick={() => onDismiss(t.id)}
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
});

/**
 * useToast returns the toast API. Callers MUST be inside a
 * ToastProvider; we throw a clear error otherwise so a missed mount
 * shows up at first call instead of silently swallowing every
 * rollback notification.
 */
export function useToast(): ToastContextShape {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error(
      "useToast must be called within a <ToastProvider> (mounted in web/src/app/main.tsx)",
    );
  }
  return ctx;
}

/**
 * useOptionalToast returns the toast API or a no-op fallback when the
 * provider is missing. Hooks that fire during boot (or in tests that
 * skip the provider) can opt into this so a missing provider doesn't
 * throw — you just lose the visual feedback.
 */
export function useOptionalToast(): ToastContextShape {
  const ctx = useContext(ToastContext);
  if (ctx) return ctx;
  return NOOP_TOAST_API;
}

const NOOP_TOAST_API: ToastContextShape = {
  push: () => {},
  error: () => {},
  success: () => {},
  info: () => {},
  dismiss: () => {},
  toasts: [],
};
