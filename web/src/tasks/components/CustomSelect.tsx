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
import type { CustomSelectOption } from "./customSelectModel";
import {
  firstSelectableIndex,
  isCustomSelectHeader,
  lastSelectableIndex,
  nextSelectable,
  prevSelectable,
} from "./customSelectModel";

export type { CustomSelectOption } from "./customSelectModel";
export { isCustomSelectHeader } from "./customSelectModel";

type Props = {
  id: string;
  label: string;
  value: string;
  options: CustomSelectOption[];
  onChange: (value: string) => void;
  className?: string;
  /** Accessible name for the listbox (defaults to `label`). */
  listboxName?: string;
  /** Tighter width for filter toolbar. */
  compact?: boolean;
  /** Shown next to the field label (default: no badge). */
  requirement?: FieldRequirement;
  disabled?: boolean;
};

export function CustomSelect({
  id,
  label,
  value,
  options,
  onChange,
  className,
  listboxName,
  compact = false,
  requirement = "none",
  disabled = false,
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
    depth?: number;
    rowTag?: string;
  } => {
    const sel = options.find(
      (
        o,
      ): o is {
        value: string;
        label: string;
        pillClass?: string;
        depth?: number;
        rowTag?: string;
      } => !isCustomSelectHeader(o) && o.value === value,
    );
    if (sel) return sel;
    const first = options.find(
      (
        o,
      ): o is {
        value: string;
        label: string;
        pillClass?: string;
        depth?: number;
        rowTag?: string;
      } => !isCustomSelectHeader(o),
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
      } else if (e.key === "Tab") {
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
    if (disabled) return;
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
    } else if (e.key === "Enter" || e.key === " ") {
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
    } else if (e.key === "Tab") {
      // Keep keyboard navigation predictable: close the popover and allow focus to move on.
      setOpen(false);
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
          /* Above modals — matches --z-portal-popover in app-design-tokens.css */
          zIndex: "var(--z-portal-popover)",
        }}
        onKeyDown={onListKeyDown}
        onBlur={() => setOpen(false)}
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
              aria-label={
                o.rowTag ? `${o.rowTag}: ${o.label}` : undefined
              }
              className={
                i === highlight
                  ? "custom-select-option custom-select-option--highlight"
                  : "custom-select-option"
              }
              style={
                o.depth != null && o.depth > 0
                  ? {
                      paddingLeft: `calc(0.35rem + ${o.depth} * 0.85rem)`,
                    }
                  : undefined
              }
              onMouseEnter={() => setHighlight(i)}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => pick(o.value)}
            >
              {o.pillClass ? (
                <span
                  className={[
                    "custom-select-option-row",
                    o.rowTag ? "custom-select-row--tagged" : "",
                  ]
                    .filter(Boolean)
                    .join(" ")}
                >
                  {o.rowTag ? (
                    <span className="custom-select-option-tag">{o.rowTag}</span>
                  ) : null}
                  <span
                    className={`custom-select-option-pill ${o.pillClass}`}
                  >
                    {o.label}
                  </span>
                </span>
              ) : (
                <span
                  className={[
                    "custom-select-option-row",
                    o.rowTag ? "custom-select-row--tagged" : "",
                  ]
                    .filter(Boolean)
                    .join(" ")}
                >
                  {o.rowTag ? (
                    <span className="custom-select-option-tag">{o.rowTag}</span>
                  ) : null}
                  <span
                    className={
                      o.depth != null && o.depth > 0
                        ? "custom-select-option-neutral custom-select-option-neutral--nested"
                        : "custom-select-option-neutral"
                    }
                  >
                    {o.label}
                  </span>
                </span>
              )}
            </li>
          ),
        )}
      </ul>
    ) : null;

  return (
    <div
      className={[
        compact
          ? "field field--custom-select field--custom-select--compact"
          : "field field--custom-select",
        className ?? "",
      ]
        .filter(Boolean)
        .join(" ")}
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
        disabled={disabled}
        onClick={() => {
          if (disabled) return;
          setOpen((o) => !o);
        }}
        onKeyDown={onButtonKeyDown}
      >
        {current.pillClass ? (
          <span
            className={[
              "custom-select-value-row",
              current.rowTag ? "custom-select-row--tagged" : "",
            ]
              .filter(Boolean)
              .join(" ")}
          >
            {current.rowTag ? (
              <span className="custom-select-value-tag">{current.rowTag}</span>
            ) : null}
            <span className={`custom-select-value-pill ${current.pillClass}`}>
              {current.label}
            </span>
          </span>
        ) : (
          <span
            className={[
              "custom-select-value-row",
              current.rowTag ? "custom-select-row--tagged" : "",
            ]
              .filter(Boolean)
              .join(" ")}
          >
            {current.rowTag ? (
              <span className="custom-select-value-tag">{current.rowTag}</span>
            ) : null}
            <span
              className={
                current.value === ""
                  ? "custom-select-value-neutral custom-select-value-neutral--placeholder"
                  : "custom-select-value-neutral"
              }
            >
              {current.label}
            </span>
          </span>
        )}
        <span className="custom-select-chevron" aria-hidden="true">
          ▾
        </span>
      </button>
      {dropdown ? createPortal(dropdown, document.body) : null}
    </div>
  );
}
