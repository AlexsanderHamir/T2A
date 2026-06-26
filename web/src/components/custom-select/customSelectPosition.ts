/** Gap between trigger and dropdown panel (px). */
export const CUSTOM_SELECT_DROPDOWN_GAP = 6;

/** Keep the panel inside the viewport with a small breathing margin. */
export const CUSTOM_SELECT_VIEWPORT_MARGIN = 12;

/** Matches `.custom-select-dropdown { max-height: min(50vh, 22rem) }`. */
export const CUSTOM_SELECT_PREFERRED_MAX_HEIGHT = 352;

/** Never render a menu shorter than this — still usable for 2–3 options. */
export const CUSTOM_SELECT_MIN_MENU_HEIGHT = 120;

export type CustomSelectDropdownPlacement = "below" | "above";

export type CustomSelectDropdownPosition = {
  left: number;
  width: number;
  maxHeight: number;
  placement: CustomSelectDropdownPlacement;
  top?: number;
  bottom?: number;
};

/**
 * Places the portaled listbox against the trigger, flipping above when
 * the viewport below is tight (e.g. model picker at the bottom of a modal).
 */
export function computeCustomSelectDropdownPosition(
  rect: Pick<DOMRect, "top" | "bottom" | "left" | "width">,
  viewportHeight = window.innerHeight,
): CustomSelectDropdownPosition {
  const preferredMax = Math.min(
    viewportHeight * 0.5,
    CUSTOM_SELECT_PREFERRED_MAX_HEIGHT,
  );

  const spaceBelow =
    viewportHeight - rect.bottom - CUSTOM_SELECT_VIEWPORT_MARGIN;
  const spaceAbove = rect.top - CUSTOM_SELECT_VIEWPORT_MARGIN;

  const maxBelow = Math.max(
    CUSTOM_SELECT_MIN_MENU_HEIGHT,
    Math.min(preferredMax, spaceBelow - CUSTOM_SELECT_DROPDOWN_GAP),
  );
  const maxAbove = Math.max(
    CUSTOM_SELECT_MIN_MENU_HEIGHT,
    Math.min(preferredMax, spaceAbove - CUSTOM_SELECT_DROPDOWN_GAP),
  );

  const openAbove =
    spaceBelow < preferredMax * 0.55 && maxAbove >= maxBelow;

  if (!openAbove) {
    return {
      placement: "below",
      top: rect.bottom + CUSTOM_SELECT_DROPDOWN_GAP,
      left: rect.left,
      width: rect.width,
      maxHeight: maxBelow,
    };
  }

  return {
    placement: "above",
    bottom: viewportHeight - rect.top + CUSTOM_SELECT_DROPDOWN_GAP,
    left: rect.left,
    width: rect.width,
    maxHeight: maxAbove,
  };
}
