/**
 * Section ids used both as DOM anchors (for the in-page nav rail)
 * and as test/select hooks. Keep in sync with SETTINGS_NAV_ITEMS in
 * SettingsPage.tsx.
 *
 * `cursorAgent`, `agentWorker`, `verification`, and `runTimeout` are
 * retained so existing deep links still scroll to a meaningful target:
 * `#cursor-agent` → Cursor runner card; `#agent-worker` → Execute
 * phase block; `#run-timeout` → max execute duration field;
 * `#verification` → Verify phase block.
 */
export const SECTION_IDS = {
  agentWorker: "agent-worker",
  cursorAgent: "cursor-agent",
  phases: "phases",
  verification: "verification",
  runTimeout: "run-timeout",
  display: "display",
  developer: "developer",
} as const;
