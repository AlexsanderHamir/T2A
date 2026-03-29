import {
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
} from "react";
import { createPortal } from "react-dom";

export type CustomSelectOption = {
  value: string;
  label: string;
  /** When set, option shows a colored pill (same system as table badges). */
  pillClass?: string;
};

type Props = {
  id: string;
  label: string;
  value: string;
  options: CustomSelectOption[];
  onChange: (value: string) => void;
  /** Accessible name for the listbox (defaults to `label`). */
  listboxName?: string;
  /** Tighter width for filter toolbar. */
  compact?: boolean;
};

export function CustomSelect({
  id,
  label,
  value,
  options,
  onChange,
  listboxName,
  compact = false,
}: Props) {
  const [open, setOpen] = useState(false);
  const [highlight, setHighlight] = useState(0);
  const [pos, setPos] = useState<{
    top: number;
    left: number;
    width: number;
  } | null>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const listboxId = useId();
  const lb = listboxName ?? label;

  const optionId = useCallback((v: string) => `${id}-opt-${v}`, [id]);

  const current = useMemo(
    () => options.find((o) => o.value === value) ?? options[0],
    [options, value],
  );

  const updatePosition = useCallback(() => {
    const el = buttonRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    setPos({ top: r.bottom + 6, left: r.left, width: r.width });
  }, []);

  useLayoutEffect(() => {
    if (!open) {
      setPos(null);
      return;
    }
    updatePosition();
    const onMove = () => updatePosition();
    window.addEventListener("scroll", onMove, true);
    window.addEventListener("resize", onMove);
    return () => {
      window.removeEventListener("scroll", onMove, true);
      window.removeEventListener("resize", onMove);
    };
  }, [open, updatePosition]);

  useEffect(() => {
    if (!open) return;
    const i = options.findIndex((o) => o.value === value);
    setHighlight(i >= 0 ? i : 0);
  }, [open, value, options]);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      const t = e.target as Node;
      if (buttonRef.current?.contains(t) || listRef.current?.contains(t))
        return;
      setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopPropagation();
        setOpen(false);
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [open]);

  useLayoutEffect(() => {
    if (open) listRef.current?.focus();
  }, [open]);

  const n = options.length;

  const pick = useCallback(
    (v: string) => {
      onChange(v);
      setOpen(false);
      buttonRef.current?.focus();
    },
    [onChange],
  );

  const onButtonKeyDown = (e: ReactKeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      if (!open) setOpen(true);
      else setHighlight((h) => (h + 1) % n);
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      if (!open) setOpen(true);
      else setHighlight((h) => (h - 1 + n) % n);
      return;
    }
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      if (open) pick(options[highlight].value);
      else setOpen(true);
    }
  };

  const onListKeyDown = (e: ReactKeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlight((h) => (h + 1) % n);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlight((h) => (h - 1 + n) % n);
    } else if (e.key === "Enter") {
      e.preventDefault();
      pick(options[highlight].value);
    } else if (e.key === "Home") {
      e.preventDefault();
      setHighlight(0);
    } else if (e.key === "End") {
      e.preventDefault();
      setHighlight(n - 1);
    } else if (e.key === "Escape") {
      e.preventDefault();
      setOpen(false);
      buttonRef.current?.focus();
    }
  };

  const highlighted = options[highlight];

  const dropdown =
    open && pos ? (
      <ul
        ref={listRef}
        id={listboxId}
        role="listbox"
        tabIndex={-1}
        aria-label={lb}
        aria-activedescendant={optionId(highlighted.value)}
        className="custom-select-dropdown"
        style={{
          position: "fixed",
          top: pos.top,
          left: pos.left,
          width: Math.max(pos.width, compact ? 10 * 16 : 12 * 16),
          zIndex: 12_000,
        }}
        onKeyDown={onListKeyDown}
      >
        {options.map((o, i) => (
          <li
            key={o.value}
            id={optionId(o.value)}
            role="option"
            aria-selected={o.value === value}
            className={
              i === highlight
                ? "custom-select-option custom-select-option--highlight"
                : "custom-select-option"
            }
            onMouseEnter={() => setHighlight(i)}
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => pick(o.value)}
          >
            {o.pillClass ? (
              <span
                className={`custom-select-option-pill ${o.pillClass}`}
              >
                {o.label}
              </span>
            ) : (
              <span className="custom-select-option-neutral">{o.label}</span>
            )}
          </li>
        ))}
      </ul>
    ) : null;

  return (
    <div
      className={
        compact
          ? "field field--custom-select field--custom-select--compact"
          : "field field--custom-select"
      }
    >
      <label htmlFor={id}>{label}</label>
      <button
        ref={buttonRef}
        type="button"
        id={id}
        role="combobox"
        className="custom-select-trigger"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listboxId}
        onClick={() => setOpen((o) => !o)}
        onKeyDown={onButtonKeyDown}
      >
        {current.pillClass ? (
          <span className={`custom-select-value-pill ${current.pillClass}`}>
            {current.label}
          </span>
        ) : (
          <span className="custom-select-value-neutral">{current.label}</span>
        )}
        <span className="custom-select-chevron" aria-hidden="true">
          ▾
        </span>
      </button>
      {dropdown ? createPortal(dropdown, document.body) : null}
    </div>
  );
}
