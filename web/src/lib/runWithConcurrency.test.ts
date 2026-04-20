import { describe, expect, it } from "vitest";
import { runWithConcurrency } from "./runWithConcurrency";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe("runWithConcurrency", () => {
  it("returns an empty array for no tasks", async () => {
    expect(await runWithConcurrency([], 5)).toEqual([]);
  });

  it("runs all tasks and preserves input order in results", async () => {
    const out = await runWithConcurrency(
      [async () => 1, async () => 2, async () => 3],
      2,
    );
    expect(out).toEqual([
      { ok: true, value: 1 },
      { ok: true, value: 2 },
      { ok: true, value: 3 },
    ]);
  });

  it("captures rejection as { ok: false, error } without throwing", async () => {
    const boom = new Error("kaboom");
    const out = await runWithConcurrency(
      [async () => "a", async () => Promise.reject(boom), async () => "c"],
      2,
    );
    expect(out).toEqual([
      { ok: true, value: "a" },
      { ok: false, error: boom },
      { ok: true, value: "c" },
    ]);
  });

  it("never has more than `limit` tasks in flight at once", async () => {
    const max = 3;
    let inFlight = 0;
    let observedMax = 0;
    const tasks = Array.from({ length: 12 }, () => async () => {
      inFlight++;
      observedMax = Math.max(observedMax, inFlight);
      await new Promise((r) => setTimeout(r, 5));
      inFlight--;
      return inFlight;
    });
    await runWithConcurrency(tasks, max);
    expect(observedMax).toBeLessThanOrEqual(max);
    expect(observedMax).toBeGreaterThan(0);
  });

  it("clamps `limit` below 1 to 1 (no division-by-zero, no idle workers)", async () => {
    const out = await runWithConcurrency(
      [async () => "x", async () => "y"],
      0,
    );
    expect(out).toEqual([
      { ok: true, value: "x" },
      { ok: true, value: "y" },
    ]);
  });

  it("clamps `limit` above task count so we don't spawn idle workers", async () => {
    const out = await runWithConcurrency([async () => 42], 100);
    expect(out).toEqual([{ ok: true, value: 42 }]);
  });

  it("processes tasks as soon as a worker is free (no batch barriers)", async () => {
    const order: number[] = [];
    const d1 = deferred<void>();
    const d2 = deferred<void>();
    const d3 = deferred<void>();
    const promise = runWithConcurrency(
      [
        async () => {
          await d1.promise;
          order.push(1);
        },
        async () => {
          await d2.promise;
          order.push(2);
        },
        async () => {
          await d3.promise;
          order.push(3);
        },
      ],
      2,
    );
    d2.resolve();
    await new Promise((r) => setTimeout(r, 0));
    d3.resolve();
    await new Promise((r) => setTimeout(r, 0));
    d1.resolve();
    await promise;
    expect(order).toEqual([2, 3, 1]);
  });
});
