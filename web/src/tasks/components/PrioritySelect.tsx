import {
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { PRIORITIES, type Priority } from "@/types";
import { priorityPillClass } from "../taskPillClasses";

type Props = {
  id: string;
  value: Priority;
  onChange: (p: Priority) => void;
};

export function PrioritySelect({ id, value, onChange }: Props) {
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
  const optionId = (p: Priority) => `${id}-opt-${p}`;

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
    const i = PRIORITIES.indexOf(value);
    setHighlight(i >= 0 ? i : 0);
  }, [open, value]);

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
    (p: Priority) => {
      onChange(p);
      setOpen(false);
      buttonRef.current?.focus();
    },
    [onChange],
  );

  const onButtonKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      if (!open) setOpen(true);
      else setHighlight((h) => (h + 1) % PRIORITIES.length);
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      if (!open) setOpen(true);
      else setHighlight((h) => (h - 1 + PRIORITIES.length) % PRIORITIES.length);
      return;
    }
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      if (open) pick(PRIORITIES[highlight]);
      else setOpen(true);
    }
  };

  const onListKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlight((h) => (h + 1) % PRIORITIES.length);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlight((h) => (h - 1 + PRIORITIES.length) % PRIORITIES.length);
    } else if (e.key === "Enter") {
      e.preventDefault();
      pick(PRIORITIES[highlight]);
    } else if (e.key === "Home") {
      e.preventDefault();
      setHighlight(0);
    } else if (e.key === "End") {
      e.preventDefault();
      setHighlight(PRIORITIES.length - 1);
    } else if (e.key === "Escape") {
      e.preventDefault();
      setOpen(false);
      buttonRef.current?.focus();
    }
  };

  const dropdown =
    open && pos ? (
      <ul
        ref={listRef}
        id={listboxId}
        role="listbox"
        tabIndex={-1}
        aria-label="Priority"
        aria-activedescendant={optionId(PRIORITIES[highlight])}
        className="custom-select-dropdown"
        style={{
          position: "fixed",
          top: pos.top,
          left: pos.left,
          width: Math.max(pos.width, 12 * 16),
          zIndex: 12_000,
        }}
        onKeyDown={onListKeyDown}
      >
        {PRIORITIES.map((p, i) => (
          <li
            key={p}
            id={optionId(p)}
            role="option"
            aria-selected={p === value}
            className={
              i === highlight
                ? "custom-select-option custom-select-option--highlight"
                : "custom-select-option"
            }
            onMouseEnter={() => setHighlight(i)}
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => pick(p)}
          >
            <span className={`custom-select-option-pill ${priorityPillClass(p)}`}>
              {p}
            </span>
          </li>
        ))}
      </ul>
    ) : null;

  return (
    <div className="field field--custom-select">
      <label htmlFor={id}>Priority</label>
      <button
        ref={buttonRef}
        type="button"
        id={id}
        className="custom-select-trigger"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listboxId}
        onClick={() => setOpen((o) => !o)}
        onKeyDown={onButtonKeyDown}
      >
        <span className={`custom-select-value-pill ${priorityPillClass(value)}`}>
          {value}
        </span>
        <span className="custom-select-chevron" aria-hidden="true">▾</span>
      </button>
      {dropdown ? createPortal(dropdown, document.body) : null}
    </div>
  );
}
