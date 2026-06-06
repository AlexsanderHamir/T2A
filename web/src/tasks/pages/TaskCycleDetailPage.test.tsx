import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "../../lib/routerFutureFlags";
import { requestUrl } from "../../test/requestUrl";
import { pushAgentRunProgress } from "../hooks/useAgentRunProgress";
import { TaskCycleDetailPage } from "./TaskCycleDetailPage";

function renderAttemptPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter
        future={ROUTER_FUTURE_FLAGS}
        initialEntries={["/tasks/t1/cycles/cyc-1"]}
      >
        <Routes>
          <Route
            path="/tasks/:taskId/cycles/:cycleId"
            element={<TaskCycleDetailPage />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const cycleDetail = {
  id: "cyc-1",
  task_id: "t1",
  attempt_seq: 3,
  status: "running",
  started_at: "2026-04-25T12:00:00.000Z",
  triggered_by: "agent",
  meta: {},
  cycle_meta: {
    runner: "cursor",
    runner_version: "1.0.0",
    cursor_model: "",
    cursor_model_effective: "auto",
    prompt_hash: "sha256:abc",
  },
  phases: [
    {
      id: "phase-2",
      cycle_id: "cyc-1",
      phase: "execute",
      phase_seq: 2,
      status: "running",
      started_at: "2026-04-25T12:00:10.000Z",
      details: {},
    },
  ],
};

function streamEvent(n: number) {
  return {
    id: `stream-${n}`,
    task_id: "t1",
    cycle_id: "cyc-1",
    phase_seq: 2,
    stream_seq: n,
    at: `2026-04-25T12:00:${String(n).padStart(2, "0")}.000Z`,
    source: "cursor",
    kind: "message",
    message: `Cursor update ${n}`,
    payload: {
      type: "assistant",
      message: { content: [{ type: "text", text: `full payload ${n}` }] },
    },
  };
}

function auditEvent(n: number) {
  return {
    seq: n,
    at: `2026-04-25T12:01:${String(n).padStart(2, "0")}.000Z`,
    type: "phase_started",
    by: "agent",
    data: { cycle_id: "cyc-1", phase_seq: 2 },
  };
}

describe("TaskCycleDetailPage", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it("bounds stream and audit timelines behind load-more controls", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/t1/cycles/cyc-1") {
        return Response.json(cycleDetail);
      }
      if (url === "/tasks/t1/cycles/cyc-1/stream?limit=500") {
        return Response.json({
          task_id: "t1",
          cycle_id: "cyc-1",
          events: Array.from({ length: 8 }, (_, i) => streamEvent(i + 1)),
          limit: 500,
          has_more: false,
        });
      }
      if (url === "/tasks/t1/events?limit=200") {
        return Response.json({
          task_id: "t1",
          events: Array.from({ length: 8 }, (_, i) => auditEvent(i + 1)),
          approval_pending: false,
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderAttemptPage();

    expect(
      await screen.findByRole("heading", { name: /attempt #3/i }),
    ).toBeInTheDocument();
    const nowSpy = vi.spyOn(Date, "now");
    nowSpy.mockReturnValue(Date.parse("2026-04-25T12:00:30.000Z"));
    act(() => {
      pushAgentRunProgress({
        taskId: "t1",
        cycleId: "cyc-1",
        phaseSeq: 2,
        progress: {
          kind: "message",
          message: "Still working live",
        },
      });
    });
    nowSpy.mockReturnValue(Date.parse("2026-04-25T12:00:35.000Z"));
    act(() => {
      pushAgentRunProgress({
        taskId: "t1",
        cycleId: "cyc-1",
        phaseSeq: 2,
        progress: {
          kind: "message",
          message: "Newest live update",
        },
      });
    });
    nowSpy.mockReturnValue(Date.parse("2026-04-25T12:00:40.000Z"));
    act(() => {
      pushAgentRunProgress({
        taskId: "t1",
        cycleId: "cyc-1",
        phaseSeq: 2,
        progress: {
          kind: "tool_call",
          subtype: "started",
          message: "Searching for *.go in worker",
        },
      });
    });
    expect(screen.getByText(/^Live$/)).toBeInTheDocument();
    expect(screen.getByText("Still working live")).toBeInTheDocument();
    expect(screen.getByText("Newest live update")).toBeInTheDocument();
    expect(screen.getByText("Searching for *.go in worker")).toBeInTheDocument();
    expect(screen.queryByText("tool_call")).toBeNull();
    const liveList = screen.getByRole("list", { name: /recent live updates/i });
    const liveItems = within(liveList).getAllByRole("listitem");
    expect(liveItems[0]).toHaveTextContent(/waiting/i);
    expect(liveItems[0]).toHaveTextContent(/last just now/i);
    expect(liveItems[1]).toHaveTextContent("Tool call");
    expect(liveItems[1]).toHaveTextContent("Searching for *.go in worker");
    expect(liveItems[2]).toHaveTextContent("Newest live update");
    expect(liveItems[3]).toHaveTextContent("Still working live");

    const activitySection = screen.getByRole("heading", {
      name: /^activity$/i,
    }).parentElement?.parentElement;
    if (!activitySection) throw new Error("missing activity section");
    expect(within(activitySection).getByText("Cursor update 8")).toBeInTheDocument();
    expect(within(activitySection).queryByText("Cursor update 1")).toBeNull();
    await user.click(within(activitySection).getByText("Cursor update 8"));
    expect(within(activitySection).getAllByText("Raw payload").length).toBeGreaterThan(0);
    expect(within(activitySection).queryByText("Full message")).toBeNull();
    expect(within(activitySection).getByText(/full payload 8/)).toBeInTheDocument();
    await user.click(within(activitySection).getByRole("button", { name: /load more/i }));
    expect(within(activitySection).getByText("Cursor update 1")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: /audit/i }));
    const auditPanel = screen.getByRole("tabpanel", { name: /audit/i });
    const auditEventLinks = within(auditPanel)
      .getAllByRole("link")
      .filter((link) => Boolean(link.getAttribute("href")?.includes("/events/")));
    expect(auditEventLinks.some((link) => link.getAttribute("href") === "/tasks/t1/events/8")).toBe(
      true,
    );
    expect(auditEventLinks.some((link) => link.getAttribute("href") === "/tasks/t1/events/1")).toBe(
      false,
    );
    await user.click(within(auditPanel).getByRole("button", { name: /load more/i }));
    const expandedAuditLinks = within(auditPanel)
      .getAllByRole("link")
      .filter((link) => Boolean(link.getAttribute("href")?.includes("/events/")));
    expect(
      expandedAuditLinks.some((link) => link.getAttribute("href") === "/tasks/t1/events/1"),
    ).toBe(true);
  });

  it("shows matching phase sequence badges in the phases track and stream", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/t1/cycles/cyc-1") {
        return Response.json({
          ...cycleDetail,
          phases: [
            {
              id: "phase-1",
              cycle_id: "cyc-1",
              phase: "execute",
              phase_seq: 1,
              status: "succeeded",
              started_at: "2026-04-25T12:00:10.000Z",
              ended_at: "2026-04-25T12:00:20.000Z",
              details: {},
            },
            {
              id: "phase-2",
              cycle_id: "cyc-1",
              phase: "verify",
              phase_seq: 2,
              status: "succeeded",
              started_at: "2026-04-25T12:00:20.000Z",
              ended_at: "2026-04-25T12:00:30.000Z",
              details: {},
            },
          ],
        });
      }
      if (url === "/tasks/t1/cycles/cyc-1/stream?limit=500") {
        return Response.json({
          task_id: "t1",
          cycle_id: "cyc-1",
          events: [streamEvent(1)],
          limit: 500,
          has_more: false,
        });
      }
      if (url === "/tasks/t1/events?limit=200") {
        return Response.json({
          task_id: "t1",
          events: [],
          approval_pending: false,
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderAttemptPage();

    expect(
      await screen.findByRole("heading", { name: /attempt #3/i }),
    ).toBeInTheDocument();

    const phasesHeading = screen.getByRole("heading", { name: /^phases$/i });
    const phasesSection = phasesHeading.closest("section");
    if (!phasesSection) throw new Error("missing phases section");
    expect(within(phasesSection).getByLabelText("Phase 1")).toHaveTextContent("PHASE 1");
    expect(within(phasesSection).getByLabelText("Phase 2")).toHaveTextContent("PHASE 2");

    const activitySection = screen.getByRole("heading", {
      name: /^activity$/i,
    }).parentElement?.parentElement;
    if (!activitySection) throw new Error("missing activity section");
    expect(within(activitySection).getByLabelText("Phase 2")).toHaveTextContent("PHASE 2");
  });

  it("updates running attempt duration on a steady timer", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(new Date("2026-04-25T12:00:30.000Z"));
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/t1/cycles/cyc-1") {
        return Response.json(cycleDetail);
      }
      if (url === "/tasks/t1/cycles/cyc-1/stream?limit=500") {
        return Response.json({
          task_id: "t1",
          cycle_id: "cyc-1",
          events: [],
          limit: 500,
          has_more: false,
        });
      }
      if (url === "/tasks/t1/events?limit=200") {
        return Response.json({
          task_id: "t1",
          events: [],
          approval_pending: false,
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderAttemptPage();
    await act(async () => {});

    expect(
      await screen.findByRole("heading", { name: /attempt #3/i }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Running for 30\.0 s/)).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(5000);
    });

    expect(screen.getByText(/Running for 35\.0 s/)).toBeInTheDocument();
  });
});
