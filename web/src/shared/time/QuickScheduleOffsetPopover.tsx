import { useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  QUICK_OFFSET_BUCKETS,
  quickOffsetChipAriaLabel,
  type QuickOffsetUnit,
} from "./quickScheduleOffsets";

type Props = {
  /**
   * Trigger button the popover anchors to. The popover positions itself
   * directly below `anchor` and falls back to above when there's not
   * enough room in the viewport. Pass `null` while the trigger is unmounted
   * (popover renders nothing in that case).
   */
  anchor: HTMLElement | null;
  onPick: (unit: QuickOffsetUnit, amount: number) => void;
  onClose: () => void;
};

const POPOVER_VERTICAL_GAP = 6;
const POPOVER_VIEWPORT_MARGIN = 12;
const POPOVER_DEFAULT_WIDTH = 340;
/**
 * Sits above modals (`--z-modal-content` floats inside the modal shell) so
 * the popover renders cleanly when triggered from inside the create-task
 * modal. Same tier as the existing CustomSelect dropdown — they should
 * never overlap because only one trigger is open at a time per modal.
 */
const POPOVER_Z_INDEX = 13000;

export type QuickScheduleOffsetPopoverProps = Props;

/**
 * Anchored popover that lists every preset offset (10 / 20 / 30 / 40 /
 * 50 minutes; 1..24 hours; 1..6 days; 1..3 weeks; 1..12 months) grouped
 * by unit. Renders into `document.body` so the modal scroll container does
 * not clip it; positions via the trigger's `getBoundingClientRect()` and
 * re-positions on viewport scroll / resize so it tracks the trigger if the
 * surrounding modal scrolls.
 *
 * The popover does NOT own the schedule state — the parent
 * `SchedulePicker` invokes `computeOffsetIso(...)` inside its `onPick`
 * handler, then closes the popover. This keeps the popover stateless and
 * trivially portable to any future surface that wants the same chip grid.
 */
export function QuickScheduleOffsetPopover({
  anchor,
  onPick,
  onClose,
}: Props) {
  const titleId = useId();
  const popoverRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{
    top: number;
    left: number;
    width: number;
    placeAbove: boolean;
  } | null>(null);

  useLayoutEffect(() => {
    if (!anchor) {
      setPos(null);
      return;
    }
    const compute = () => {
      const rect = anchor.getBoundingClientRect();
      const popHeight = popoverRef.current?.offsetHeight ?? 0;
      const viewportH = window.innerHeight;
      const viewportW = window.innerWidth;
      // Prefer below; fall back to above when there isn't enough room.
      const spaceBelow = viewportH - rect.bottom - POPOVER_VERTICAL_GAP;
      const placeAbove =
        popHeight > 0 &&
        spaceBelow < popHeight &&
        rect.top - POPOVER_VERTICAL_GAP > popHeight;
      const width = Math.min(
        POPOVER_DEFAULT_WIDTH,
        viewportW - 2 * POPOVER_VIEWPORT_MARGIN,
      );
      const left = Math.max(
        POPOVER_VIEWPORT_MARGIN,
        Math.min(viewportW - width - POPOVER_VIEWPORT_MARGIN, rect.left),
      );
      const top = placeAbove
        ? Math.max(POPOVER_VIEWPORT_MARGIN, rect.top - popHeight - POPOVER_VERTICAL_GAP)
        : rect.bottom + POPOVER_VERTICAL_GAP;
      setPos({ top, left, width, placeAbove });
    };
    compute();
    const onResize = () => compute();
    window.addEventListener("scroll", onResize, true);
    window.addEventListener("resize", onResize);
    return () => {
      window.removeEventListener("scroll", onResize, true);
      window.removeEventListener("resize", onResize);
    };
  }, [anchor]);

  // Click-outside dismiss. The trigger button is excluded so toggling the
  // popover via the same button works (the trigger's own onClick already
  // handles open ↔ close).
  useEffect(() => {
    function onDocMouseDown(e: MouseEvent) {
      const target = e.target;
      if (!(target instanceof Node)) return;
      if (popoverRef.current?.contains(target)) return;
      if (anchor?.contains(target)) return;
      onClose();
    }
    document.addEventListener("mousedown", onDocMouseDown);
    return () => document.removeEventListener("mousedown", onDocMouseDown);
  }, [anchor, onClose]);

  // Escape closes the popover and returns focus to the trigger.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key !== "Escape") return;
      e.preventDefault();
      e.stopPropagation();
      onClose();
      anchor?.focus();
    }
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [anchor, onClose]);

  // Focus the popover on open so screen readers announce the dialog title
  // and keyboard users can immediately Tab into the chip grid.
  useLayoutEffect(() => {
    if (!pos) return;
    popoverRef.current?.focus();
  }, [pos]);

  if (!anchor) return null;

  return createPortal(
    <div
      ref={popoverRef}
      role="dialog"
      aria-labelledby={titleId}
      aria-modal="false"
      tabIndex={-1}
      data-testid="schedule-picker-quick-popover"
      className="schedule-picker-quick-popover"
      data-place-above={pos?.placeAbove ? "true" : "false"}
      style={{
        position: "fixed",
        top: pos?.top ?? -9999,
        left: pos?.left ?? -9999,
        width: pos?.width ?? POPOVER_DEFAULT_WIDTH,
        // Keep the popover invisible (but rendered) on the first frame so we
        // can measure `offsetHeight` and decide above/below before the
        // operator sees a flash at the top-left corner.
        visibility: pos ? "visible" : "hidden",
        zIndex: POPOVER_Z_INDEX,
      }}
    >
      <header className="schedule-picker-quick-popover__header">
        <h3 id={titleId} className="schedule-picker-quick-popover__title">
          Schedule for later
        </h3>
        <p className="schedule-picker-quick-popover__hint">
          Pick a delay from now.
        </p>
      </header>
      <div className="schedule-picker-quick-popover__sections">
        {QUICK_OFFSET_BUCKETS.map((bucket) => (
          <section
            key={bucket.unit}
            className="schedule-picker-quick-popover__section"
            data-unit={bucket.unit}
            aria-labelledby={`${titleId}-${bucket.unit}-label`}
          >
            <h4
              id={`${titleId}-${bucket.unit}-label`}
              className="schedule-picker-quick-popover__section-label"
            >
              {bucket.label}
            </h4>
            <div className="schedule-picker-quick-popover__chips">
              {bucket.amounts.map((amount) => (
                <button
                  key={amount}
                  type="button"
                  className="schedule-picker-quick-popover__chip"
                  data-testid={`schedule-picker-quick-${bucket.unit}-${amount}`}
                  aria-label={quickOffsetChipAriaLabel(bucket, amount)}
                  onClick={() => onPick(bucket.unit, amount)}
                >
                  {bucket.formatChip(amount)}
                </button>
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>,
    document.body,
  );
}
