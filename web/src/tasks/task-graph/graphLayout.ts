/**
 * Pixel geometry for `TaskGraphPage` virtualization (canvas size, culling, edges).
 * Must match `--task-graph-*` in `app-design-tokens.css` and `.task-graph-node` in
 * `app-task-detail.css` (graph math assumes **1rem = 16px**).
 */
export const GRAPH_LAYOUT_PX = {
  CARD_WIDTH: 240,
  CARD_HEIGHT: 104,
  COL_GAP: 88,
  ROW_GAP: 20,
  PADDING: 24,
  BUFFER_ROWS: 20,
} as const;
