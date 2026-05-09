import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import { requestUrl } from "@/test/requestUrl";
import { ProjectStepsEntryCard } from "./ProjectStepsEntryCard";

type FetchInput = RequestInfo | URL;

function jsonResponse(body: unknown, init: ResponseInit = { status: 200 }): Response {
  return new Response(JSON.stringify(body), {
    ...init,
    headers: { "content-type": "application/json", ...(init.headers ?? {}) },
  });
}

describe("ProjectStepsEntryCard", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("links to the project steps route and surfaces the step count", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: FetchInput) => {
      const u = requestUrl(input);
      if (u.endsWith("/projects/proj-1/steps")) {
        return jsonResponse({
          steps: [
            {
              id: "step-1",
              project_id: "proj-1",
              title: "Discovery",
              description: "",
              sort_order: 1,
              gate_status: "active",
              gate_hold: false,
              criteria: [],
              created_at: "2026-01-01T00:00:00Z",
              updated_at: "2026-01-01T00:00:00Z",
            },
          ],
        });
      }
      return new Response("not found", { status: 404 });
    });

    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false, gcTime: 0 },
        mutations: { retry: false },
      },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
          <ProjectStepsEntryCard projectId="proj-1" />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    const link = screen.getByRole("link", { name: "Steps" });
    expect(link).toHaveAttribute("href", "/projects/proj-1/steps");
    await waitFor(() => {
      expect(screen.getByText("1 stage")).toBeInTheDocument();
    });
  });
});
