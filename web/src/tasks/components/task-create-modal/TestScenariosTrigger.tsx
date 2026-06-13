import { forwardRef } from "react";

type Props = {
  open: boolean;
  disabled?: boolean;
  onToggle: () => void;
};

/**
 * Header-mounted trigger that opens the test-scenarios popover. Lives in
 * its own component so the create modal can keep a clean ref to the
 * underlying button (the popover anchors to it via getBoundingClientRect).
 *
 * Mirrors the schedule picker's "Schedule for later…" trigger styling so
 * both popover triggers in the modal feel like siblings.
 */
export const TestScenariosTrigger = forwardRef<HTMLButtonElement, Props>(
  function TestScenariosTrigger({ open, disabled, onToggle }, ref) {
    return (
      <button
        ref={ref}
        type="button"
        className="test-scenarios-trigger"
        data-testid="test-scenarios-trigger"
        data-active={open ? "true" : "false"}
        aria-haspopup="dialog"
        aria-expanded={open}
        disabled={disabled}
        onClick={onToggle}
      >
        <SparkleGlyph />
        <span className="test-scenarios-trigger__label">Test scenarios</span>
        <ChevronGlyph />
      </button>
    );
  },
);

function ChevronGlyph() {
  return (
    <svg
      className="test-scenarios-trigger-chevron"
      width="12"
      height="12"
      viewBox="0 0 12 12"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M3 4.5L6 7.5L9 4.5" />
    </svg>
  );
}

function SparkleGlyph() {
  // 4-point sparkle — same shorthand for "AI / preset / suggested" used by
  // the model picker glyph elsewhere in the modal. Stroke-only so it
  // inherits color from the trigger button's text color.
  return (
    <svg
      width="13"
      height="13"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.4"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M8 2.25L9.35 6.4 13.5 7.75 9.35 9.1 8 13.25 6.65 9.1 2.5 7.75 6.65 6.4z" />
      <path d="M12.5 2.25v2" />
      <path d="M11.5 3.25h2" />
    </svg>
  );
}
