import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { requestUrl } from "../test/requestUrl";
import { ObservabilityPage } from "./ObservabilityPage";

function jsonResponse(body: unknown, init: ResponseInit = { status: 200 }): Response {
  return new Response(JSON.stringify(body), {
    ...init,
    headers: { "content-type": "application/json", ...(init.headers ?? {}) },
  });
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <ObservabilityPage />
    </QueryClientProvider>,
  );
}

describe("ObservabilityPage", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("loads /tasks/stats and renders the overview", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const u = requestUrl(input);
      if (u.endsWith("/tasks/stats")) {
        return jsonResponse({
          total: 4,
          ready: 1,
          critical: 1,
          by_status: { ready: 1, done: 2, failed: 1 },
          by_priority: { medium: 3, critical: 1 },
          by_scope: { parent: 2, subtask: 2 },
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderPage();

    expect(
      await screen.findByRole("heading", { name: "Observability" }),
    ).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByTestId("obs-kpi-total")).toHaveTextContent("4");
    });
    expect(screen.getByTestId("obs-kpi-done")).toHaveTextContent("2");
    expect(screen.getByTestId("obs-kpi-failed")).toHaveTextContent("1");
  });

  it("renders the unavailable state when the stats request fails", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (requestUrl(input).endsWith("/tasks/stats")) {
        return new Response("boom", { status: 500 });
      }
      return new Response("not found", { status: 404 });
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getAllByText("—").length).toBeGreaterThan(0);
    });
    expect(screen.getByText("Breakdown unavailable")).toBeInTheDocument();
  });
});
