import { useEffect, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { listFocusables } from "./modalFocus";
import { useModalStackOptional, type ModalEscapeRef } from "./ModalStackContext";

type Props = {
  children: ReactNode;
  onClose: () => void;
  /** Matches `id` on the dialog heading for `aria-labelledby`. */
  labelledBy: string;
  /** Optional `id` (or space-separated ids) for supplementary dialog copy, e.g. a lead paragraph. */
  describedBy?: string;
  /** Wider shell for forms with rich text (default ~narrow confirm). */
  size?: "default" | "wide";
  /** Shows a blocking spinner overlay; backdrop and Escape are disabled by default while busy. */
  busy?: boolean;
  /** `aria-label` on the busy spinner (screen readers). */
  busyLabel?: string;
  /**
   * When true, the user can still close the modal via Escape or
   * backdrop click *even while* `busy` is true (the spinner overlay
   * keeps showing in-flight feedback). Use for long-running
   * background operations the user shouldn't be trapped behind
   * (e.g. a slow create that the caller has already made safe to
   * resolve after the modal has closed). Defaults to `false` so
   * existing call sites keep the historical "modal locks while
   * busy" behavior.
   */
  dismissibleWhileBusy?: boolean;
  /** When false, nested modals avoid fighting the parent on `document.body` scroll lock. */
  lockBodyScroll?: boolean;
  /** Higher stacking when opened above another modal. */
  stack?: "default" | "nested";
};

export function Modal({
  children,
  onClose,
  labelledBy,
  describedBy,
  size = "default",
  busy = false,
  busyLabel = "Saving…",
  dismissibleWhileBusy = false,
  lockBodyScroll = true,
  stack = "default",
}: Props) {
  const modalStack = useModalStackOptional();
  const escapeRef = useRef({
    busy,
    dismissibleWhileBusy,
    onClose,
  }) as ModalEscapeRef;
  escapeRef.current = { busy, dismissibleWhileBusy, onClose };

  const rootRef = useRef<HTMLDivElement>(null);
  const shellRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const root = rootRef.current;
    const shell = shellRef.current;
    if (!root || !shell) return;

    const previous =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;

    const raf = requestAnimationFrame(() => {
      if (busy) return;
      if (shell.contains(document.activeElement)) return;
      const list = listFocusables(shell);
      if (list.length > 0) {
        list[0]?.focus();
      } else {
        shell.focus();
      }
    });

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== "Tab" || busy) return;
      const active = document.activeElement;
      if (!(active instanceof Node) || !root.contains(active)) return;

      const list = listFocusables(shell);
      if (list.length === 0) return;

      const first = list[0]!;
      const last = list[list.length - 1]!;

      if (e.shiftKey) {
        if (active === first) {
          e.preventDefault();
          last.focus();
        }
      } else if (active === last) {
        e.preventDefault();
        first.focus();
      }
    };

    document.addEventListener("keydown", onKeyDown, true);

    return () => {
      cancelAnimationFrame(raf);
      document.removeEventListener("keydown", onKeyDown, true);
      if (previous?.isConnected && typeof previous.focus === "function") {
        previous.focus();
      }
    };
  }, [busy, labelledBy]);

  useEffect(() => {
    if (!lockBodyScroll) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, [lockBodyScroll]);

  useEffect(() => {
    if (modalStack) {
      modalStack.register(escapeRef);
      return () => modalStack.unregister(escapeRef);
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      const r = escapeRef.current;
      if (r.busy && !r.dismissibleWhileBusy) return;
      r.onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [modalStack]);

  const rootClass =
    stack === "nested" ? "modal-root modal-root--nested" : "modal-root";

  return createPortal(
    <div className={rootClass} ref={rootRef}>
      <div
        className="modal-backdrop"
        aria-hidden="true"
        onClick={() => {
          if (!busy || dismissibleWhileBusy) onClose();
        }}
      />
      <div
        ref={shellRef}
        className={
          size === "wide" ? "modal-shell modal-shell--wide" : "modal-shell"
        }
        role="dialog"
        aria-modal="true"
        aria-labelledby={labelledBy}
        {...(describedBy ? { "aria-describedby": describedBy } : {})}
        aria-busy={busy}
        tabIndex={-1}
      >
        {children}
        {busy ? (
          <div className="modal-busy-overlay">
            <div
              className="modal-spinner"
              role="status"
              aria-label={busyLabel}
            />
          </div>
        ) : null}
      </div>
    </div>,
    document.body,
  );
}
