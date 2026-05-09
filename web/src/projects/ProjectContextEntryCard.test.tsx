import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import { requestUrl } from "@/test/requestUrl";
import { ProjectContextEntryCard } from "./ProjectContextEntryCard";

type FetchInput = RequestInfo | URL;

function jsonResponse(body: unknown, init: ResponseInit = { status: 200 }): Response {
  return new Response(JSON.stringify(body), {
    ...init,
    headers: { "content-type": "application/json", ...(init.headers ?? {}) },
  });
}

describe("ProjectContextEntryCard", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("links to the project context route and surfaces the node count", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: FetchInput) => {
      const u = requestUrl(input);
      if (/\/projects\/proj-1\/context\?/.test(u)) {
        return jsonResponse({
          items: [
            {
              id: "11111111-1111-4111-8111-111111111111",
              project_id: "proj-1",
              kind: "constraint",
              title: "A",
              body: "B",
              created_by: "user",
              pinned: false,
              created_at: "2026-01-01T00:00:00Z",
              updated_at: "2026-01-01T00:00:00Z",
            },
          ],
          edges: [],
          limit: 100,
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
          <ProjectContextEntryCard projectId="proj-1" />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    const link = screen.getByRole("link", { name: /Project context/ });
    expect(link).toHaveAttribute("href", "/projects/proj-1/context");
    await waitFor(() => {
      expect(screen.getByText("1 node")).toBeInTheDocument();
    });
  });
});
