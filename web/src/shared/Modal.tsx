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
};

export function Modal({
  children,
  onClose,
  labelledBy,
  size = "default",
  busy = false,
}: Props) {
  useEffect(() => {
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose, busy]);

  return createPortal(
    <div className="modal-root">
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
              aria-label="Saving…"
            />
          </div>
        ) : null}
      </div>
    </div>,
    document.body,
  );
}
