import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import { stubEventSource } from "../test/browserMocks";
import { requestUrl } from "../test/requestUrl";

function renderApp() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>,
  );
}

describe("App", () => {
  beforeEach(() => {
    stubEventSource();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("renders heading and empty state after tasks load", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/repo/")) {
        return new Response(
          JSON.stringify({ error: "repo not configured" }),
          { status: 503 },
        );
      }
      return new Response("not found", { status: 404 });
    });

    renderApp();
    expect(
      await screen.findByRole("heading", { name: /^tasks$/i }),
    ).toBeInTheDocument();
    expect(await screen.findByText("No tasks yet.")).toBeInTheDocument();
  });

  it("shows an alert when the initial list request fails", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("network down"));

    renderApp();

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("network down");
  });

  it("creates a task and shows it in the table after refresh", async () => {
    const user = userEvent.setup();
    let created = false;

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        if (!created) {
          return Response.json({ tasks: [], limit: 200, offset: 0 });
        }
        return Response.json({
          tasks: [
            {
              id: "t1",
              title: "Ship fix",
              initial_prompt: "",
              status: "ready",
              priority: "medium",
            },
          ],
          limit: 200,
          offset: 0,
        });
      }
      if (url.startsWith("/repo/")) {
        return new Response(
          JSON.stringify({ error: "repo not configured" }),
          { status: 503 },
        );
      }
      if (url === "/tasks" && init?.method === "POST") {
        created = true;
        return new Response(
          JSON.stringify({
            id: "t1",
            title: "Ship fix",
            initial_prompt: "",
            status: "ready",
            priority: "medium",
          }),
          { status: 201, headers: { "Content-Type": "application/json" } },
        );
      }
      return new Response("not found", { status: 404 });
    });

    renderApp();
    await screen.findByText("No tasks yet.");

    await user.type(screen.getByLabelText(/^title$/i), "Ship fix");
    await user.click(screen.getByRole("button", { name: /^create$/i }));

    expect(await screen.findByText("Ship fix")).toBeInTheDocument();
  });
});
