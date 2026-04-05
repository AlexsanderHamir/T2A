import { useEffect, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { useModalStackOptional, type ModalEscapeRef } from "./ModalStackContext";

function focusableSelector(): string {
  return [
    "a[href]:not([disabled])",
    "button:not([disabled])",
    "textarea:not([disabled])",
    'input:not([disabled]):not([type="hidden"])',
    "select:not([disabled])",
    '[tabindex]:not([tabindex="-1"])',
  ].join(", ");
}

function listFocusables(shell: HTMLElement): HTMLElement[] {
  return Array.from(shell.querySelectorAll<HTMLElement>(focusableSelector())).filter(
    (el) => !el.closest("[aria-hidden='true']"),
  );
}

type Props = {
  children: ReactNode;
  onClose: () => void;
  /** Matches `id` on the dialog heading for `aria-labelledby`. */
  labelledBy: string;
  /** Wider shell for forms with rich text (default ~narrow confirm). */
  size?: "default" | "wide";
  /** Shows a blocking spinner overlay; backdrop and Escape are disabled. */
  busy?: boolean;
  /** `aria-label` on the busy spinner (screen readers). */
  busyLabel?: string;
  /** When false, nested modals avoid fighting the parent on `document.body` scroll lock. */
  lockBodyScroll?: boolean;
  /** Higher stacking when opened above another modal. */
  stack?: "default" | "nested";
};

export function Modal({
  children,
  onClose,
  labelledBy,
  size = "default",
  busy = false,
  busyLabel = "Saving…",
  lockBodyScroll = true,
  stack = "default",
}: Props) {
  const modalStack = useModalStackOptional();
  const escapeRef = useRef({ busy, onClose }) as ModalEscapeRef;
  escapeRef.current = { busy, onClose };

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
      if (e.key !== "Escape" || escapeRef.current.busy) return;
      escapeRef.current.onClose();
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
          if (!busy) onClose();
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
