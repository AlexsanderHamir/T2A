import { useEffect, type ReactNode } from "react";
import { createPortal } from "react-dom";

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
  useEffect(() => {
    if (!lockBodyScroll) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, [lockBodyScroll]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose, busy]);

  const rootClass =
    stack === "nested" ? "modal-root modal-root--nested" : "modal-root";

  return createPortal(
    <div className={rootClass}>
      <div
        className="modal-backdrop"
        aria-hidden="true"
        onClick={() => {
          if (!busy) onClose();
        }}
      />
      <div
        className={
          size === "wide" ? "modal-shell modal-shell--wide" : "modal-shell"
        }
        role="dialog"
        aria-modal="true"
        aria-labelledby={labelledBy}
        aria-busy={busy}
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
