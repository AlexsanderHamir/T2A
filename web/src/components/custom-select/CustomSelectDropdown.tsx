import { forwardRef, type KeyboardEvent as ReactKeyboardEvent } from "react";
import type { CustomSelectOption } from "./customSelectModel";
import { isCustomSelectHeader } from "./customSelectModel";
import { CustomSelectRowBody } from "./CustomSelectRowBody";
import type { CustomSelectDropdownPosition } from "./customSelectPosition";

export type { CustomSelectDropdownPosition } from "./customSelectPosition";

type Props = {
  listboxId: string;
  listboxAriaLabel: string;
  value: string;
  options: CustomSelectOption[];
  highlight: number;
  compact: boolean;
  dropdownMinWidth?: number;
  dropdownVariant?: "default" | "toolbar";
  ariaActivedescendant?: string;
  optionId: (v: string) => string;
  pos: CustomSelectDropdownPosition;
  onListKeyDown: (e: ReactKeyboardEvent<HTMLUListElement>) => void;
  onClose: () => void;
  onHighlightIndex: (index: number) => void;
  onPick: (value: string) => void;
};

export const CustomSelectDropdown = forwardRef<HTMLUListElement, Props>(
  function CustomSelectDropdown(
    {
      listboxId,
      listboxAriaLabel,
      value,
      options,
      highlight,
      compact,
      dropdownMinWidth,
      dropdownVariant = "default",
      ariaActivedescendant,
      optionId,
      pos,
      onListKeyDown,
      onClose,
      onHighlightIndex,
      onPick,
    },
    ref,
  ) {
    const minWidthPx =
      dropdownMinWidth ?? (compact ? 13 * 16 : 12 * 16);

    return (
      <ul
        ref={ref}
        id={listboxId}
        role="listbox"
        tabIndex={-1}
        aria-label={listboxAriaLabel}
        aria-activedescendant={ariaActivedescendant}
        className={[
          "custom-select-dropdown",
          dropdownVariant === "toolbar"
            ? "custom-select-dropdown--toolbar"
            : "",
        ]
          .filter(Boolean)
          .join(" ")}
        style={{
          position: "fixed",
          ...(pos.placement === "above"
            ? { bottom: pos.bottom }
            : { top: pos.top }),
          left: pos.left,
          width: Math.max(pos.width, minWidthPx),
          maxHeight: pos.maxHeight,
          /* Above --z-modal (11000); see app-design-tokens-foundation.css */
          zIndex: "var(--z-portal-popover)",
        }}
        onKeyDown={onListKeyDown}
        onBlur={onClose}
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
              aria-label={o.rowTag ? `${o.rowTag}: ${o.label}` : undefined}
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
              onMouseEnter={() => onHighlightIndex(i)}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => onPick(o.value)}
            >
              <CustomSelectRowBody
                variant="option"
                rowTag={o.rowTag}
                label={o.label}
                pillClass={o.pillClass}
                depth={o.depth}
              />
            </li>
          ),
        )}
      </ul>
    );
  },
);
