import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TaskCyclesPanel } from "./TaskCyclesPanel";
import { pushAgentRunProgress } from "../../../hooks/useAgentRunProgress";

/**
 * The panel composes useTaskCycles + useTaskCycle and renders five
 * distinct states:
 *   1. loading
 *   2. error
 *   3. empty (no cycles ever recorded)
 *   4. populated, no running cycle (history only, no live ticker)
 *   5. populated with running cycle (live ticker + history; phase
 *      detail fetched per row)
 *
 * We drive the states through fetch mocks rather than mocking the
 * hooks themselves so the parsing layer (api/cycles.ts +
 * parseTaskApi.ts) is exercised by the test too — protecting against
 * a parser regression silently breaking the panel.
 */

type FetchInput = Parameters<typeof fetch>[0];

function createWrapper() {
  const qc = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, staleTime: 0 },
    },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

const okJSON = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });

const reqUrl = (input: FetchInput): string =>
  typeof input === "string"
    ? input
    : input instanceof URL
      ? input.toString()
      : (input as Request).url;

function renderPanel(taskId = "task-1") {
  const Wrapper = createWrapper();
  return render(
    <Wrapper>
      <TaskCyclesPanel taskId={taskId} />
    </Wrapper>,
  );
}

describe("TaskCyclesPanel", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders a loading skeleton while the cycles list query is pending", async () => {
    // fetch never resolves → query stays pending forever (test-scoped).
    vi.spyOn(globalThis, "fetch").mockImplementation(
      () => new Promise(() => {}),
    );
    const { container } = renderPanel();

    // The skeleton list must be busy-announced for assistive tech.
    const busy = container.querySelector('[aria-busy="true"]');
    expect(busy).not.toBeNull();
    expect(busy?.getAttribute("aria-label")).toMatch(/Loading execution cycles/i);
    // No live ticker yet: we don't know if there's a running cycle.
    expect(screen.queryByTestId("task-cycle-ticker")).not.toBeInTheDocument();
  });

  it("surfaces an error with a retry control when the cycles fetch fails", async () => {
    let callCount = 0;
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async () => {
        callCount += 1;
        if (callCount === 1) {
          return new Response(
            JSON.stringify({ error: "boom" }),
            { status: 500, headers: { "Content-Type": "application/json" } },
          );
        }
        return okJSON({
          task_id: "task-1",
          cycles: [],
          limit: 50,
          has_more: false,
        });
      });

    renderPanel();

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(/boom/);

    // Retry button refetches and the error gives way to the empty state.
    await userEvent.click(screen.getByRole("button", { name: /Try again/i }));
    await screen.findByText(/No execution cycles yet/i);
    expect(fetchSpy).toHaveBeenCalledTimes(2);
  });

  it("renders the empty state when the task has no cycles", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      okJSON({
        task_id: "task-1",
        cycles: [],
        limit: 50,
        has_more: false,
      }),
    );

    renderPanel();

    await screen.findByText(/No execution cycles yet/i);
    // No ticker, no list, no error — just the empty state.
    expect(screen.queryByTestId("task-cycle-ticker")).not.toBeInTheDocument();
    expect(screen.queryByTestId("task-cycles-list")).not.toBeInTheDocument();
  });

  it("lists historical cycles newest-first and lazy-loads phases on row expansion", async () => {
    // Two terminal cycles, no running one. Row expansion triggers
    // a per-cycle detail fetch that we count below to assert the
    // panel doesn't waste bandwidth on collapsed rows.
    const detailCalls: string[] = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = reqUrl(input);
      if (url.endsWith("/tasks/task-1/cycles")) {
        return okJSON({
          task_id: "task-1",
          cycles: [
            {
              id: "cyc-2",
              task_id: "task-1",
              attempt_seq: 2,
              status: "succeeded",
              started_at: "2026-04-18T11:00:00.000Z",
              ended_at: "2026-04-18T11:01:00.000Z",
              triggered_by: "agent",
              meta: {},
            },
            {
              id: "cyc-1",
              task_id: "task-1",
              attempt_seq: 1,
              status: "failed",
              started_at: "2026-04-18T10:00:00.000Z",
              ended_at: "2026-04-18T10:00:45.000Z",
              triggered_by: "user",
              meta: {},
            },
          ],
          limit: 50,
          has_more: false,
        });
      }
      if (url.startsWith("/tasks/task-1/cycles/")) {
        detailCalls.push(url);
        const id = url.replace("/tasks/task-1/cycles/", "");
        return okJSON({
          id,
          task_id: "task-1",
          attempt_seq: id === "cyc-2" ? 2 : 1,
          status: id === "cyc-2" ? "succeeded" : "failed",
          started_at: "2026-04-18T10:00:00.000Z",
          ended_at: "2026-04-18T10:00:45.000Z",
          triggered_by: "agent",
          meta: {},
          phases: [
            {
              id: `${id}-ph-1`,
              cycle_id: id,
              phase: "diagnose",
              phase_seq: 1,
              status: "succeeded",
              started_at: "2026-04-18T10:00:01.000Z",
              ended_at: "2026-04-18T10:00:10.000Z",
              details: {},
              summary: "looked at the failure",
            },
            {
              id: `${id}-ph-2`,
              cycle_id: id,
              phase: "execute",
              phase_seq: 2,
              status: id === "cyc-2" ? "succeeded" : "failed",
              started_at: "2026-04-18T10:00:11.000Z",
              ended_at: "2026-04-18T10:00:40.000Z",
              details: {},
            },
          ],
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderPanel();

    // List shows both cycles; the running ticker is absent.
    const list = await screen.findByTestId("task-cycles-list");
    const items = within(list).getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(within(items[0]).getByText(/Attempt #2/)).toBeInTheDocument();
    expect(within(items[1]).getByText(/Attempt #1/)).toBeInTheDocument();
    expect(screen.queryByTestId("task-cycle-ticker")).not.toBeInTheDocument();

    // Phase fetch is lazy — collapsed rows must not have hit /cycles/{id}.
    expect(detailCalls).toEqual([]);

    // Expanding the first row triggers exactly one detail fetch.
    await userEvent.click(within(items[0]).getByText(/Attempt #2/));
    await waitFor(() => expect(detailCalls).toEqual(["/tasks/task-1/cycles/cyc-2"]));
    await within(items[0]).findByText(/looked at the failure/);

    // Expanding the second row triggers a second, distinct detail fetch.
    await userEvent.click(within(items[1]).getByText(/Attempt #1/));
    await waitFor(() =>
      expect(detailCalls).toEqual([
        "/tasks/task-1/cycles/cyc-2",
        "/tasks/task-1/cycles/cyc-1",
      ]),
    );
  });

  it("renders the live ticker for the running cycle with the currently running phase", async () => {
    // Freeze Date.now so the elapsed-time string is stable.
    const fakeNow = Date.parse("2026-04-18T11:00:30.000Z");
    vi.spyOn(Date, "now").mockReturnValue(fakeNow);

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = reqUrl(input);
      if (url.endsWith("/tasks/task-1/cycles")) {
        return okJSON({
          task_id: "task-1",
          cycles: [
            {
              id: "cyc-live",
              task_id: "task-1",
              attempt_seq: 3,
              status: "running",
              started_at: "2026-04-18T11:00:00.000Z",
              triggered_by: "agent",
              meta: {},
            },
          ],
          limit: 50,
          has_more: false,
        });
      }
      if (url === "/tasks/task-1/cycles/cyc-live") {
        return okJSON({
          id: "cyc-live",
          task_id: "task-1",
          attempt_seq: 3,
          status: "running",
          started_at: "2026-04-18T11:00:00.000Z",
          triggered_by: "agent",
          meta: {},
          phases: [
            {
              id: "p-d",
              cycle_id: "cyc-live",
              phase: "diagnose",
              phase_seq: 1,
              status: "succeeded",
              started_at: "2026-04-18T11:00:01.000Z",
              ended_at: "2026-04-18T11:00:10.000Z",
              details: {},
            },
            {
              id: "p-e",
              cycle_id: "cyc-live",
              phase: "execute",
              phase_seq: 2,
              status: "running",
              started_at: "2026-04-18T11:00:11.000Z",
              details: {},
            },
          ],
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderPanel();

    const ticker = await screen.findByTestId("task-cycle-ticker");
    // Cycle is running and live region is polite.
    expect(ticker.getAttribute("aria-live")).toBe("polite");
    expect(within(ticker).getByTestId("task-cycle-ticker-status")).toHaveTextContent(
      /Running/,
    );
    expect(within(ticker).getByText(/Attempt #3/)).toBeInTheDocument();
    // Started 30 s ago at fake-now (11:00:30 - 11:00:00).
    expect(
      within(ticker).getByTestId("task-cycle-ticker-elapsed"),
    ).toHaveTextContent(/Started 30\.0 s ago/);

    // The phase line resolves to the running execute phase.
    const phaseLine = await within(ticker).findByTestId(
      "task-cycle-ticker-phase",
    );
    expect(phaseLine).toHaveTextContent(/Now running/);
    expect(phaseLine).toHaveTextContent(/Execute/);
    // Phase started 19 s ago (11:00:30 - 11:00:11 = 19 s).
    expect(phaseLine).toHaveTextContent(/for 19\.0 s/);

    // The running cycle ALSO appears in the history list, with a
    // small "↑ live" hint pointing the user up to the ticker.
    const list = screen.getByTestId("task-cycles-list");
    expect(within(list).getByLabelText(/shown in the live ticker above/i)).toBeInTheDocument();
  });

  it("renders a runner/model chip on each cycle row and the live ticker (Phase 4b of plan)", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = reqUrl(input);
      if (url.endsWith("/tasks/task-1/cycles")) {
        return okJSON({
          task_id: "task-1",
          cycles: [
            {
              id: "cyc-live",
              task_id: "task-1",
              attempt_seq: 2,
              status: "running",
              started_at: "2026-04-18T11:00:00.000Z",
              triggered_by: "agent",
              meta: {},
              cycle_meta: {
                runner: "cursor",
                runner_version: "v1.2.3",
                cursor_model: "opus-4",
                cursor_model_effective: "opus-4",
                prompt_hash: "abc",
              },
            },
            {
              id: "cyc-hist",
              task_id: "task-1",
              attempt_seq: 1,
              status: "succeeded",
              started_at: "2026-04-18T10:00:00.000Z",
              ended_at: "2026-04-18T10:01:00.000Z",
              triggered_by: "user",
              meta: {},
              cycle_meta: {
                runner: "cursor",
                runner_version: "v1.2.3",
                cursor_model: "",
                cursor_model_effective: "sonnet-4.5",
                prompt_hash: "def",
              },
            },
          ],
          limit: 50,
          has_more: false,
        });
      }
      if (url === "/tasks/task-1/cycles/cyc-live") {
        return okJSON({
          id: "cyc-live",
          task_id: "task-1",
          attempt_seq: 2,
          status: "running",
          started_at: "2026-04-18T11:00:00.000Z",
          triggered_by: "agent",
          meta: {},
          cycle_meta: {
            runner: "cursor",
            runner_version: "v1.2.3",
            cursor_model: "opus-4",
            cursor_model_effective: "opus-4",
            prompt_hash: "abc",
          },
          phases: [],
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderPanel();

    const ticker = await screen.findByTestId("task-cycle-ticker");
    expect(within(ticker).getByTestId("task-cycle-ticker-runner")).toHaveTextContent(
      "Cursor CLI · opus-4",
    );

    const list = screen.getByTestId("task-cycles-list");
    const rowRunners = within(list).getAllByTestId("task-cycle-row-runner");
    // Both rows render a runner chip (running cycle is also in history).
    expect(rowRunners.length).toBeGreaterThanOrEqual(2);
    // The terminal history cycle resolved to sonnet-4.5 even though
    // the task's intent was empty — chip reads the effective value.
    expect(rowRunners.map((el) => el.textContent)).toEqual(
      expect.arrayContaining(["Cursor CLI · opus-4", "Cursor CLI · sonnet-4.5"]),
    );
  });

  it("falls back to a 'between phases' line when the running cycle has no in-flight phase", async () => {
    // Cycle is running but every phase has already terminated —
    // the worker is between StartCycle/StartPhase frames. The
    // ticker must still resolve gracefully and show the most
    // recent (highest phase_seq) phase rather than going blank.
    const fakeNow = Date.parse("2026-04-18T11:01:00.000Z");
    vi.spyOn(Date, "now").mockReturnValue(fakeNow);

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = reqUrl(input);
      if (url.endsWith("/tasks/task-1/cycles")) {
        return okJSON({
          task_id: "task-1",
          cycles: [
            {
              id: "cyc-tween",
              task_id: "task-1",
              attempt_seq: 4,
              status: "running",
              started_at: "2026-04-18T11:00:00.000Z",
              triggered_by: "agent",
              meta: {},
            },
          ],
          limit: 50,
          has_more: false,
        });
      }
      if (url === "/tasks/task-1/cycles/cyc-tween") {
        return okJSON({
          id: "cyc-tween",
          task_id: "task-1",
          attempt_seq: 4,
          status: "running",
          started_at: "2026-04-18T11:00:00.000Z",
          triggered_by: "agent",
          meta: {},
          phases: [
            {
              id: "p-d",
              cycle_id: "cyc-tween",
              phase: "diagnose",
              phase_seq: 1,
              status: "succeeded",
              started_at: "2026-04-18T11:00:01.000Z",
              ended_at: "2026-04-18T11:00:10.000Z",
              details: {},
            },
            {
              id: "p-e",
              cycle_id: "cyc-tween",
              phase: "execute",
              phase_seq: 2,
              status: "succeeded",
              started_at: "2026-04-18T11:00:11.000Z",
              ended_at: "2026-04-18T11:00:40.000Z",
              details: {},
            },
          ],
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderPanel();

    const phaseLine = await screen.findByTestId("task-cycle-ticker-phase");
    expect(phaseLine).toHaveTextContent(/Between phases/);
    expect(phaseLine).toHaveTextContent(/Execute/);
    expect(phaseLine).toHaveTextContent(/succeeded/);
  });

  it("renders bounded live progress under the running phase", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = reqUrl(input);
      if (url.endsWith("/tasks/task-1/cycles")) {
        return okJSON({
          task_id: "task-1",
          cycles: [
            {
              id: "cyc-live-progress",
              task_id: "task-1",
              attempt_seq: 1,
              status: "running",
              started_at: "2026-04-18T11:00:00.000Z",
              triggered_by: "agent",
              meta: {},
            },
          ],
          limit: 50,
          has_more: false,
        });
      }
      if (url === "/tasks/task-1/cycles/cyc-live-progress") {
        return okJSON({
          id: "cyc-live-progress",
          task_id: "task-1",
          attempt_seq: 1,
          status: "running",
          started_at: "2026-04-18T11:00:00.000Z",
          triggered_by: "agent",
          meta: {},
          phases: [
            {
              id: "p-e",
              cycle_id: "cyc-live-progress",
              phase: "execute",
              phase_seq: 2,
              status: "running",
              started_at: "2026-04-18T11:00:05.000Z",
              details: {},
            },
          ],
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderPanel();

    const ticker = await screen.findByTestId("task-cycle-ticker");
    expect(await within(ticker).findByTestId("task-cycle-progress-empty")).toHaveTextContent(
      /Waiting for the next agent update/,
    );

    act(() => {
      pushAgentRunProgress({
        taskId: "task-1",
        cycleId: "cyc-live-progress",
        phaseSeq: 2,
        progress: {
          kind: "tool_call",
          subtype: "started",
          tool: "ReadFile",
          message: "Started ReadFile",
        },
      });
    });

    const progressList = await within(ticker).findByTestId("task-cycle-progress-list");
    expect(progressList).toHaveAttribute("aria-label", "Recent agent progress");
    expect(progressList).toHaveTextContent(/Tool/);
    expect(progressList).toHaveTextContent(/Started ReadFile/);
  });
});
