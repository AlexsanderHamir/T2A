import { describe, expect, it } from "vitest";
import {
  CUSTOM_SELECT_DROPDOWN_GAP,
  computeCustomSelectDropdownPosition,
} from "./customSelectPosition";

function rect(
  top: number,
  height: number,
  left = 100,
  width = 200,
): Pick<DOMRect, "top" | "bottom" | "left" | "width"> {
  return { top, left, width, bottom: top + height };
}

describe("computeCustomSelectDropdownPosition", () => {
  it("opens below with a viewport-capped max height when space is ample", () => {
    const pos = computeCustomSelectDropdownPosition(rect(200, 44), 900);
    expect(pos.placement).toBe("below");
    expect(pos.top).toBe(200 + 44 + CUSTOM_SELECT_DROPDOWN_GAP);
    expect(pos.maxHeight).toBeGreaterThan(200);
  });

  it("flips above when the trigger sits near the bottom of the viewport", () => {
    const pos = computeCustomSelectDropdownPosition(rect(820, 44), 900);
    expect(pos.placement).toBe("above");
    expect(pos.bottom).toBe(900 - 820 + CUSTOM_SELECT_DROPDOWN_GAP);
    expect(pos.top).toBeUndefined();
    expect(pos.maxHeight).toBeGreaterThan(120);
  });

  it("limits max height to available space below the trigger", () => {
    const pos = computeCustomSelectDropdownPosition(rect(400, 44), 900);
    expect(pos.placement).toBe("below");
    expect(pos.maxHeight).toBeLessThanOrEqual(
      900 - 400 - 44 - 12 - CUSTOM_SELECT_DROPDOWN_GAP + 1,
    );
  });
});
