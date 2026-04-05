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
import {
  FieldRequirementBadge,
  type FieldRequirement,
} from "@/shared/FieldLabel";

export type CustomSelectOption =
  | { type: "header"; label: string }
  | { value: string; label: string; pillClass?: string };

export function isCustomSelectHeader(
  o: CustomSelectOption,
): o is { type: "header"; label: string } {
  return "type" in o && o.type === "header";
}

function firstSelectableIndex(opts: CustomSelectOption[]): number {
  const i = opts.findIndex((o) => !isCustomSelectHeader(o));
  return i >= 0 ? i : 0;
}

function lastSelectableIndex(opts: CustomSelectOption[]): number {
  for (let i = opts.length - 1; i >= 0; i--) {
    if (!isCustomSelectHeader(opts[i])) return i;
  }
  return 0;
}

function nextSelectable(
  opts: CustomSelectOption[],
  from: number,
): number {
  for (let i = from + 1; i < opts.length; i++) {
    if (!isCustomSelectHeader(opts[i])) return i;
  }
  for (let i = 0; i < opts.length; i++) {
    if (!isCustomSelectHeader(opts[i])) return i;
  }
  return from;
}

function prevSelectable(
  opts: CustomSelectOption[],
  from: number,
): number {
  for (let i = from - 1; i >= 0; i--) {
    if (!isCustomSelectHeader(opts[i])) return i;
  }
  for (let i = opts.length - 1; i >= 0; i--) {
    if (!isCustomSelectHeader(opts[i])) return i;
  }
  return from;
}

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
  /** Shown next to the field label (default: no badge). */
  requirement?: FieldRequirement;
};

export function CustomSelect({
  id,
  label,
  value,
  options,
  onChange,
  listboxName,
  compact = false,
  requirement = "none",
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

  const current = useMemo((): {
    value: string;
    label: string;
    pillClass?: string;
  } => {
    const sel = options.find(
      (o): o is { value: string; label: string; pillClass?: string } =>
        !isCustomSelectHeader(o) && o.value === value,
    );
    if (sel) return sel;
    const first = options.find(
      (o): o is { value: string; label: string; pillClass?: string } =>
        !isCustomSelectHeader(o),
    );
    return first ?? { value: "", label: "" };
  }, [options, value]);

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
    const i = options.findIndex(
      (o) => !isCustomSelectHeader(o) && o.value === value,
    );
    setHighlight(i >= 0 ? i : firstSelectableIndex(options));
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
      else setHighlight((h) => nextSelectable(options, h));
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      if (!open) setOpen(true);
      else setHighlight((h) => prevSelectable(options, h));
      return;
    }
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      if (open) {
        const o = options[highlight];
        if (!isCustomSelectHeader(o)) pick(o.value);
      } else setOpen(true);
    }
  };

  const onListKeyDown = (e: ReactKeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlight((h) => nextSelectable(options, h));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlight((h) => prevSelectable(options, h));
    } else if (e.key === "Enter") {
      e.preventDefault();
      const o = options[highlight];
      if (!isCustomSelectHeader(o)) pick(o.value);
    } else if (e.key === "Home") {
      e.preventDefault();
      setHighlight(firstSelectableIndex(options));
    } else if (e.key === "End") {
      e.preventDefault();
      setHighlight(lastSelectableIndex(options));
    } else if (e.key === "Escape") {
      e.preventDefault();
      setOpen(false);
      buttonRef.current?.focus();
    }
  };

  const highlighted = options[highlight];
  const highlightedOption =
    highlighted && !isCustomSelectHeader(highlighted) ? highlighted : null;

  const dropdown =
    open && pos ? (
      <ul
        ref={listRef}
        id={listboxId}
        role="listbox"
        tabIndex={-1}
        aria-label={lb}
        aria-activedescendant={
          highlightedOption ? optionId(highlightedOption.value) : undefined
        }
        className="custom-select-dropdown"
        style={{
          position: "fixed",
          top: pos.top,
          left: pos.left,
          width: Math.max(pos.width, compact ? 10 * 16 : 12 * 16),
          /* Above .modal-root (11k) and .modal-root--nested (12050); list is portaled to body. */
          zIndex: 13000,
        }}
        onKeyDown={onListKeyDown}
      >
        {options.map((o, i) =>
          isCustomSelectHeader(o) ? (
            <li
              key={`header-${i}-${o.label}`}
              role="presentation"
              className="custom-select-option-header"
            >
              {o.label}
            </li>
          ) : (
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
          ),
        )}
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
      <div className="field-label-with-req">
        <label htmlFor={id}>{label}</label>
        <FieldRequirementBadge requirement={requirement} />
      </div>
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
