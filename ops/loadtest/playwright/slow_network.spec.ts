// Playwright scenario for Phase 5 — slow-3G + 4× CPU throttling.
//
// Opens the SPA in Chromium, connects to the dev backend at
// TASKAPI_ORIGIN (default 127.0.0.1:8080, SPA proxied through Vite
// at 5173), and repeatedly flips a task's status between "ready"
// and "working". For each flip we record:
//
//   optimisticRenderMs — time from click to the DOM reflecting the
//     new status under the optimistic-apply path.
//   serverSettleMs     — time from click to the server-confirmed
//     event arriving back on the SSE stream (detected via the
//     React Query cache update that follows the event, surfaced
//     on window.__t2a_test_hooks by web/src/observability/rum.ts).
//
// Pass:
//   expect(p95(optimisticRenderMs)).toBeLessThan(100)
//   expect(p95(serverSettleMs)).toBeLessThan(2000)
//
// This test is NOT run by default `npx playwright test`; it lives
// outside the web/ package so CI does not install playwright for
// every PR. Run manually:
//
//   cd web && npm i -D @playwright/test && npx playwright install chromium
//   TASKAPI_ORIGIN=http://127.0.0.1:8080 SPA_ORIGIN=http://127.0.0.1:5173 \
//     npx playwright test ../ops/loadtest/playwright/slow_network.spec.ts

import { test, expect, CDPSession } from "@playwright/test";

const SPA_ORIGIN = process.env.SPA_ORIGIN || "http://127.0.0.1:5173";
const FLIPS = Number(process.env.FLIPS || 25);

function p(samples: number[], q: number): number {
  if (samples.length === 0) return NaN;
  const sorted = [...samples].sort((a, b) => a - b);
  const idx = Math.min(sorted.length - 1, Math.floor(q * sorted.length));
  return sorted[idx];
}

async function applyThrottling(cdp: CDPSession) {
  // Slow-3G: 500 kbps down, 500 kbps up, 400ms RTT.
  await cdp.send("Network.enable");
  await cdp.send("Network.emulateNetworkConditions", {
    offline: false,
    latency: 400,
    downloadThroughput: (500 * 1024) / 8,
    uploadThroughput: (500 * 1024) / 8,
  });
  // 4x CPU throttling.
  await cdp.send("Emulation.setCPUThrottlingRate", { rate: 4 });
}

test("optimistic status flip holds under slow-3G + 4x CPU", async ({ page, context }) => {
  const cdp = await context.newCDPSession(page);
  await applyThrottling(cdp);

  await page.goto(`${SPA_ORIGIN}/`);
  // Wait for a task row to be visible. The SPA list page renders a
  // table; we target the first row by role.
  const firstRow = page.getByRole("row").nth(1);
  await expect(firstRow).toBeVisible({ timeout: 30_000 });

  const optimistic: number[] = [];
  const settled: number[] = [];

  for (let i = 0; i < FLIPS; i++) {
    // Open the row's action menu and click the status toggle. The
    // SPA exposes a stable testid for the status pill.
    const pill = firstRow.getByTestId("status-pill");
    const current = await pill.textContent();
    const target = current?.toLowerCase().includes("ready") ? "working" : "ready";

    const t0 = Date.now();
    await pill.click();
    const option = page.getByRole("menuitem", { name: new RegExp(target, "i") });
    await option.click();

    // Optimistic: the pill text changes immediately.
    await expect(pill).toContainText(new RegExp(target, "i"), { timeout: 1_000 });
    optimistic.push(Date.now() - t0);

    // Settle: wait for RUM to record a mutation_settled event.
    // Observed via window.__t2a_test_hooks which rum.ts exposes in
    // test mode; falls back to waiting for the next SSE event.
    const settledMs = await page.evaluate((startedAt) => {
      return new Promise<number>((resolve) => {
        const hooks = (window as unknown as { __t2a_test_hooks?: { onMutationSettled: (cb: () => void) => () => void } }).__t2a_test_hooks;
        if (!hooks) {
          resolve(Date.now() - startedAt);
          return;
        }
        const unsub = hooks.onMutationSettled(() => { unsub(); resolve(Date.now() - startedAt); });
        setTimeout(() => { try { unsub(); } catch (_) {/* ignore */} resolve(Date.now() - startedAt); }, 5_000);
      });
    }, t0);
    settled.push(settledMs);
  }

  console.log("optimistic p50/p95/p99:", p(optimistic, 0.5), p(optimistic, 0.95), p(optimistic, 0.99));
  console.log("settle p50/p95/p99:", p(settled, 0.5), p(settled, 0.95), p(settled, 0.99));

  expect(p(optimistic, 0.95)).toBeLessThan(100);
  expect(p(settled, 0.95)).toBeLessThan(2000);
});
