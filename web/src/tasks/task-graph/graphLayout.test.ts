import { describe, expect, it } from "vitest";
import { GRAPH_LAYOUT_PX } from "./graphLayout";

/**
 * Expected px at **16px/rem**, matching `:root` in `app-design-tokens.css`:
 * - `--task-graph-card-width` = 15rem
 * - `--task-graph-card-min-height` = 6.5rem
 * - `--task-graph-col-gap` = 5.5rem
 * - `--task-graph-row-gap` = var(--space-5) → 1.25rem
 * - `--task-graph-canvas-padding` = var(--space-6) → 1.5rem
 *
 * If tokens change, update both `graphLayout.ts` and this table.
 */
const TOKEN_DERIVED_PX = {
  CARD_WIDTH: 15 * 16,
  CARD_HEIGHT: 6.5 * 16,
  COL_GAP: 5.5 * 16,
  ROW_GAP: 1.25 * 16,
  PADDING: 1.5 * 16,
} as const;

describe("graphLayout", () => {
  it("GRAPH_LAYOUT_PX matches design-token rem geometry at 16px/rem", () => {
    expect({
      CARD_WIDTH: GRAPH_LAYOUT_PX.CARD_WIDTH,
      CARD_HEIGHT: GRAPH_LAYOUT_PX.CARD_HEIGHT,
      COL_GAP: GRAPH_LAYOUT_PX.COL_GAP,
      ROW_GAP: GRAPH_LAYOUT_PX.ROW_GAP,
      PADDING: GRAPH_LAYOUT_PX.PADDING,
    }).toEqual(TOKEN_DERIVED_PX);
  });

  it("BUFFER_ROWS is a virtualization constant (not from CSS tokens)", () => {
    expect(GRAPH_LAYOUT_PX.BUFFER_ROWS).toBe(20);
  });
});
