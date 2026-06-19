import { useCallback, useState } from "react";

function truncateMiddle(value: string, maxLen: number): string {
  if (value.length <= maxLen) return value;
  const head = Math.ceil((maxLen - 1) * 0.55);
  const tail = maxLen - 1 - head;
  return `${value.slice(0, head)}…${value.slice(-tail)}`;
}

type CopyableIdProps = {
  /** Full identifier (UUID, request id, etc.) */
  value: string;
  /** When true (default), shorten long values in the layout; full string stays in title + copy. */
  truncate?: boolean;
  /** Visible code text; full `value` is still copied. */
  displayValue?: string;
  /** Button label before copy succeeds. */
  copyLabel?: string;
  className?: string;
};

/**
 * Compact display for long IDs with copy-to-clipboard; keeps monospace only on the value.
 */
export function CopyableId({
  value,
  truncate = true,
  displayValue,
  copyLabel = "Copy",
  className = "",
}: CopyableIdProps) {
  const [copied, setCopied] = useState(false);
  const display =
    displayValue ??
    (truncate && value.length > 24 ? truncateMiddle(value, 22) : value);

  const copy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      /* clipboard denied — ignore */
    }
  }, [value]);

  return (
    <span className={`copyable-id ${className}`.trim()}>
      <code className="copyable-id__value" title={value}>
        {display}
      </code>
      <button
        type="button"
        className="btn-utility copyable-id__btn"
        onClick={() => void copy()}
        aria-label={
          copied
            ? "Copied to clipboard"
            : copyLabel === "Copy"
              ? "Copy full value"
              : copyLabel
        }
      >
        {copied ? "Copied" : copyLabel}
      </button>
    </span>
  );
}
