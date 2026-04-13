import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { stubEventSource } from "../../test/browserMocks";
import { requestUrl } from "../../test/requestUrl";
import { ROUTER_FUTURE_FLAGS } from "../../lib/routerFutureFlags";
import { TaskGraphPage } from "./TaskGraphPage";

function renderGraph(initialPath: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={[initialPath]}>
        <Routes>
          <Route path="/tasks/:taskId/graph" element={<TaskGraphPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("TaskGraphPage", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("shows graph skeleton while task data is loading", async () => {
    stubEventSource();
    vi.stubEnv("VITE_TASK_GRAPH_MOCK_URL", "");
    let resolveLoad!: (value: Response) => void;
    const loadPromise = new Promise<Response>((resolve) => {
      resolveLoad = resolve;
    });
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/groot") {
        return loadPromise;
      }
      return new Response("not found", { status: 404 });
    });

    renderGraph("/tasks/groot/graph");

    expect(screen.getByRole("status", { name: /loading task graph/i })).toBeInTheDocument();
    expect(document.querySelector(".task-graph-skeleton-viewport")).not.toBeNull();

    resolveLoad(
      Response.json({
        id: "groot",
        title: "Graph root",
        initial_prompt: "",
        status: "ready",
        priority: "high",
        checklist_inherit: false,
        children: [],
      }),
    );

    expect(await screen.findByRole("heading", { name: /task graph/i })).toBeInTheDocument();
    expect(screen.getByText(/1 node rendered/i)).toBeInTheDocument();
  });

  it("shows error with retry and refetches on success", async () => {
    const user = userEvent.setup();
    stubEventSource();
    vi.stubEnv("VITE_TASK_GRAPH_MOCK_URL", "");
    let calls = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/groot") {
        calls += 1;
        if (calls === 1) {
          return new Response("fail", { status: 500 });
        }
        return Response.json({
          id: "groot",
          title: "Graph root",
          initial_prompt: "",
          status: "ready",
          priority: "high",
          checklist_inherit: false,
          children: [],
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderGraph("/tasks/groot/graph");

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /try again/i }));

    expect(await screen.findByRole("heading", { name: /task graph/i })).toBeInTheDocument();
    expect(calls).toBe(2);
  });

  it("renders virtualized graph with node count and links", async () => {
    stubEventSource();
    vi.stubEnv("VITE_TASK_GRAPH_MOCK_URL", "");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/groot") {
        return Response.json({
          id: "groot",
          title: "Graph root",
          initial_prompt: "",
          status: "ready",
          priority: "high",
          checklist_inherit: false,
          children: [
            {
              id: "c1",
              title: "Child one",
              initial_prompt: "",
              status: "running",
              priority: "medium",
              checklist_inherit: false,
              children: [],
            },
          ],
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderGraph("/tasks/groot/graph");

    expect(await screen.findByRole("heading", { name: /task graph/i })).toBeInTheDocument();
    expect(screen.getByText(/2 nodes rendered/i)).toBeInTheDocument();
    const canvas = screen.getByRole("region", {
      name: /virtualized task graph canvas/i,
    });
    expect(within(canvas).getByRole("link", { name: "Graph root" })).toHaveAttribute(
      "href",
      "/tasks/groot",
    );
  });

  it("loads graph from mock URL when env is set", async () => {
    stubEventSource();
    vi.stubEnv("VITE_TASK_GRAPH_MOCK_URL", "/mock-data/graphs/task-graph-200k.json");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/mock-data/graphs/task-graph-200k.json") {
        return Response.json({
          id: "mock-root",
          title: "Mock root",
          status: "ready",
          priority: "high",
          children: [
            {
              id: "mock-child",
              title: "Mock child",
              status: "running",
              priority: "medium",
              children: [],
            },
          ],
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderGraph("/tasks/ignored/graph");

    expect(await screen.findByRole("heading", { name: /task graph/i })).toBeInTheDocument();
    expect(screen.getByText(/2 nodes rendered/i)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Mock root" })).toHaveAttribute(
      "href",
      "/tasks/mock-root",
    );
  });
});
