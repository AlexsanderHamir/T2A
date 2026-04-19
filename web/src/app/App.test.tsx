import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter, MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "../lib/routerFutureFlags";
import { DEFAULT_DOCUMENT_TITLE } from "../shared/useDocumentTitle";
import App from "./App";
import { stubEventSource } from "../test/browserMocks";
import { requestUrl } from "../test/requestUrl";

async function openNewTaskModal(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: /^new task$/i }));
  return screen.findByRole("dialog");
}

async function choosePriorityInDialog(
  user: ReturnType<typeof userEvent.setup>,
  dialog: HTMLElement,
  level: "low" | "medium" | "high" | "critical" = "medium",
) {
  const combo = within(dialog).getByRole("combobox", {
    name: /^priority$/i,
  });
  await user.click(combo);
  await user.click(
    screen.getByRole("option", { name: new RegExp(`^${level}$`, "i") }),
  );
}

function renderApp() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter future={ROUTER_FUTURE_FLAGS}>
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

  it("exposes T2A wordmark as home link with aria-current on /", async () => {
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
    await screen.findByRole("heading", { name: /^t2a$/i });
    const titleLink = screen.getByRole("link", { name: /^t2a$/i });
    expect(titleLink).toHaveAttribute("href", "/");
    expect(titleLink).toHaveAttribute("aria-current", "page");
  });

  it("navigates home when T2A wordmark is used from a task route", async () => {
    const user = userEvent.setup();
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url === "/tasks/h1") {
        return Response.json({
          id: "h1",
          title: "Home link task",
          initial_prompt: "",
          status: "ready",
          priority: "medium",
          checklist_inherit: false,
        });
      }
      if (url === "/tasks/h1/checklist") {
        return Response.json({ items: [] });
      }
      if (url.startsWith("/tasks/h1/events")) {
        return Response.json({
          task_id: "h1",
          events: [],
          limit: 20,
          total: 0,
          has_more_newer: false,
          has_more_older: false,
          approval_pending: false,
        });
      }
      if (url.startsWith("/repo/")) {
        return new Response(
          JSON.stringify({ error: "repo not configured" }),
          { status: 503 },
        );
      }
      return new Response("not found", { status: 404 });
    });

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter
          future={ROUTER_FUTURE_FLAGS}
          initialEntries={["/tasks/h1"]}
        >
          <App />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await screen.findByRole("heading", { name: /^home link task$/i });
    const titleLink = screen.getByRole("link", { name: /^t2a$/i });
    expect(titleLink).not.toHaveAttribute("aria-current");

    await user.click(titleLink);
    expect(await screen.findByText("No tasks yet")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /^t2a$/i })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("shows not found for unknown routes", async () => {
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

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter
          future={ROUTER_FUTURE_FLAGS}
          initialEntries={["/no-such-page"]}
        >
          <App />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(
      await screen.findByRole("heading", { name: /^page not found$/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /^← all tasks$/i }),
    ).toHaveAttribute("href", "/");
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
    const skip = screen.getByRole("link", { name: /^skip to main content$/i });
    expect(skip).toHaveAttribute("href", "#main-content");
    expect(screen.getByRole("main")).toHaveAttribute("id", "main-content");
    expect(
      await screen.findByRole("heading", { name: /^t2a$/i }),
    ).toBeInTheDocument();
    expect(await screen.findByText("No tasks yet")).toBeInTheDocument();
    await waitFor(() => {
      expect(document.title).toBe(DEFAULT_DOCUMENT_TITLE);
    });
    expect(document.querySelector(".route-announcer")).toHaveTextContent(
      DEFAULT_DOCUMENT_TITLE,
    );
  });

  it("shows an alert when the initial list request fails", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("network down"));

    renderApp();

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("network down");
  });

  it("shows evaluate error without unhandled rejection when draft evaluation fails", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url === "/tasks/evaluate" && init?.method === "POST") {
        return new Response(
          JSON.stringify({ error: "evaluate failed" }),
          { status: 500, headers: { "Content-Type": "application/json" } },
        );
      }
      if (url.startsWith("/task-drafts?")) {
        return Response.json({ drafts: [] });
      }
      if (url === "/task-drafts" && init?.method === "POST") {
        return new Response(
          JSON.stringify({ id: "d1", name: "Untitled draft" }),
          { status: 201, headers: { "Content-Type": "application/json" } },
        );
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
    await screen.findByText("No tasks yet");
    const dialog = await openNewTaskModal(user);
    await user.type(within(dialog).getByLabelText(/^title$/i), "Evaluate me");
    await choosePriorityInDialog(user, dialog);
    await user.click(within(dialog).getByRole("button", { name: /^evaluate$/i }));

    // The error now surfaces both globally (in the page-level
    // ErrorBanner above <main>) and inside the dialog (via the new
    // `evaluateError` prop landed alongside the checklist + subtask
    // error UX). Scope the assertion to the dialog so it pins the
    // user-visible feedback path that lives ON TOP of the modal
    // backdrop — without that, the global banner is hidden behind
    // the modal and the in-dialog callout is the user's only signal.
    const dialogAlert = await within(dialog).findByRole("alert");
    expect(dialogAlert).toHaveTextContent(/evaluate failed/i);
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
              checklist_inherit: false,
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
            checklist_inherit: false,
          }),
          { status: 201, headers: { "Content-Type": "application/json" } },
        );
      }
      if (url === "/tasks/t1/checklist/items" && init?.method === "POST") {
        return new Response(null, { status: 204 });
      }
      return new Response("not found", { status: 404 });
    });

    renderApp();
    await screen.findByText("No tasks yet");

    const dialog = await openNewTaskModal(user);
    await user.type(within(dialog).getByLabelText(/^title$/i), "Ship fix");
    await choosePriorityInDialog(user, dialog);
    await user.click(
      within(dialog).getByRole("button", { name: /^create$/i }),
    );

    expect(
      await screen.findByRole("link", { name: /ship fix/i }),
    ).toBeInTheDocument();
  });

  it("keeps draft autosave failures inside the modal", async () => {
    const user = userEvent.setup();

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        return Response.json({ drafts: [] });
      }
      if (url === "/task-drafts" && init?.method === "POST") {
        return new Response(JSON.stringify({ error: "Not Found" }), {
          status: 404,
          headers: { "Content-Type": "application/json" },
        });
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
    await screen.findByText("No tasks yet");

    const dialog = await openNewTaskModal(user);
    await user.type(within(dialog).getByLabelText(/^title$/i), "Autosave test");

    expect(
      await within(dialog).findByText(
        /Draft autosave failed\. You can still create the task\./i,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("does not submit manual save when draft has no changes", async () => {
    const user = userEvent.setup();
    const draftSaves: string[] = [];

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        return Response.json({ drafts: [] });
      }
      if (url === "/task-drafts" && init?.method === "POST") {
        draftSaves.push(String(init.body ?? ""));
        return new Response(JSON.stringify({ error: "Not Found" }), {
          status: 404,
          headers: { "Content-Type": "application/json" },
        });
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
    await screen.findByText("No tasks yet");

    const dialog = await openNewTaskModal(user);
    await user.click(within(dialog).getByRole("button", { name: /^save draft$/i }));

    expect(draftSaves).toHaveLength(0);
    expect(
      within(dialog).queryByText(
        /Draft autosave failed\. You can still create the task\./i,
      ),
    ).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("clears prior autosave error when create modal is reopened", async () => {
    const user = userEvent.setup();

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        return Response.json({ drafts: [] });
      }
      if (url === "/task-drafts" && init?.method === "POST") {
        return new Response(JSON.stringify({ error: "Not Found" }), {
          status: 404,
          headers: { "Content-Type": "application/json" },
        });
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
    await screen.findByText("No tasks yet");

    const firstDialog = await openNewTaskModal(user);
    await user.type(within(firstDialog).getByLabelText(/^title$/i), "trigger autosave");
    expect(
      await within(firstDialog).findByText(
        /Draft autosave failed\. You can still create the task\./i,
      ),
    ).toBeInTheDocument();

    await user.click(within(firstDialog).getByRole("button", { name: /^cancel$/i }));

    const secondDialog = await openNewTaskModal(user);
    expect(
      within(secondDialog).queryByText(
        /Draft autosave failed\. You can still create the task\./i,
      ),
    ).toBeNull();
  });

  it("opens a draft from drafts page in a prefilled create modal", async () => {
    const user = userEvent.setup();
    const draftSaves: string[] = [];

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        return Response.json({
          drafts: [
            {
              id: "d1",
              name: "Draft from list",
              created_at: "2026-04-07T10:00:00Z",
              updated_at: "2026-04-07T10:05:00Z",
            },
          ],
        });
      }
      if (url === "/task-drafts/d1") {
        return Response.json({
          id: "d1",
          name: "Draft from list",
          created_at: "2026-04-07T10:00:00Z",
          updated_at: "2026-04-07T10:05:00Z",
          payload: {
            title: "Prefilled title",
            initial_prompt: "Prefilled prompt",
            priority: "high",
            task_type: "feature",
            parent_id: "",
            checklist_inherit: false,
            checklist_items: ["Do step A"],
            pending_subtasks: [
              {
                title: "Child A",
                initial_prompt: "child prompt",
                priority: "medium",
                task_type: "general",
                checklist_items: ["child criterion"],
                checklist_inherit: false,
              },
            ],
            latest_evaluation: {
              overall_score: 87,
              overall_summary: "Good scope",
              sections: [{ key: "clarity", score: 92 }],
            },
          },
        });
      }
      if (url === "/task-drafts" && init?.method === "POST") {
        draftSaves.push(String(init.body ?? ""));
        return new Response(
          JSON.stringify({
            id: "d1",
            name: "Draft from list",
            created_at: "2026-04-07T10:00:00Z",
            updated_at: "2026-04-07T10:06:00Z",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }
      if (url.startsWith("/repo/")) {
        return new Response(
          JSON.stringify({ error: "repo not configured" }),
          { status: 503 },
        );
      }
      return new Response("not found", { status: 404 });
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={["/drafts"]}>
          <App />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await screen.findByRole("heading", { name: /^task drafts$/i });
    await user.click(
      await screen.findByRole("button", {
        name: /^open draft draft from list in create form$/i,
      }),
    );

    const dialog = await screen.findByRole("dialog", { name: /^new task$/i });
    // The standalone draft-name field was removed: the task title now
    // doubles as the draft name. Confirm there's no draft-name input
    // and the title field reflects the resumed payload.
    expect(within(dialog).queryByLabelText(/^draft name$/i)).not.toBeInTheDocument();
    expect(within(dialog).getByLabelText(/^title$/i)).toHaveValue("Prefilled title");
    expect(within(dialog).getByText("Do step A")).toBeInTheDocument();
    expect(within(dialog).getByText("Child A")).toBeInTheDocument();
    expect(within(dialog).getByText(/Good scope/i)).toBeInTheDocument();
    expect(draftSaves).toHaveLength(0);
  });

  it("shows loading status on drafts page while drafts are fetching", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        return new Promise<Response>(() => {});
      }
      if (url.startsWith("/repo/")) {
        return new Response(
          JSON.stringify({ error: "repo not configured" }),
          { status: 503 },
        );
      }
      return new Response("not found", { status: 404 });
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={["/drafts"]}>
          <App />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(await screen.findByRole("heading", { name: /^task drafts$/i })).toBeInTheDocument();
    expect(
      await screen.findByRole("status", { name: /loading drafts/i }),
    ).toBeInTheDocument();
  });

  it("shows an error on drafts page when draft list request fails", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        return new Response(
          JSON.stringify({ error: "drafts unavailable" }),
          { status: 500, headers: { "Content-Type": "application/json" } },
        );
      }
      if (url.startsWith("/repo/")) {
        return new Response(
          JSON.stringify({ error: "repo not configured" }),
          { status: 503 },
        );
      }
      return new Response("not found", { status: 404 });
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={["/drafts"]}>
          <App />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(/drafts unavailable/i);
    expect(
      screen.getByRole("button", { name: /^try again$/i }),
    ).toBeInTheDocument();
  });

  it("retries draft list from drafts page after an error", async () => {
    const user = userEvent.setup();
    let draftAttempts = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        draftAttempts += 1;
        if (draftAttempts === 1) {
          return new Response(
            JSON.stringify({ error: "drafts unavailable" }),
            { status: 500, headers: { "Content-Type": "application/json" } },
          );
        }
        return Response.json({
          drafts: [
            {
              id: "d1",
              name: "Recovered",
              created_at: "2026-04-07T10:00:00Z",
              updated_at: "2026-04-07T10:05:00Z",
            },
          ],
        });
      }
      if (url.startsWith("/repo/")) {
        return new Response(
          JSON.stringify({ error: "repo not configured" }),
          { status: 503 },
        );
      }
      return new Response("not found", { status: 404 });
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={["/drafts"]}>
          <App />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(/drafts unavailable/i);
    await user.click(screen.getByRole("button", { name: /^try again$/i }));
    expect(
      await screen.findByRole("button", {
        name: /open draft recovered in create form/i,
      }),
    ).toBeInTheDocument();
  });

  it("offers create task from drafts page when there are no drafts", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        return Response.json({ drafts: [] });
      }
      if (url.startsWith("/repo/")) {
        return new Response(
          JSON.stringify({ error: "repo not configured" }),
          { status: 503 },
        );
      }
      return new Response("not found", { status: 404 });
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={["/drafts"]}>
          <App />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await screen.findByRole("heading", { name: /^task drafts$/i });
    await user.click(
      await screen.findByRole("button", { name: /^create a task$/i }),
    );
    expect(
      await screen.findByRole("dialog", { name: /^new task$/i }),
    ).toBeInTheDocument();
  });

  it("shows loading status in draft picker modal from home", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        return new Promise<Response>(() => {});
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
    await screen.findByText("No tasks yet");
    await user.click(screen.getByRole("button", { name: /^new task$/i }));
    expect(await screen.findByRole("status")).toHaveTextContent(/loading drafts/i);
  });

  it("shows home entry hint when drafts fail and opens fresh create form", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        return new Response(
          JSON.stringify({ error: "drafts unavailable" }),
          { status: 500, headers: { "Content-Type": "application/json" } },
        );
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
    await screen.findByText("No tasks yet");
    await user.click(screen.getByRole("button", { name: /^new task$/i }));

    expect(await screen.findByRole("dialog", { name: /^new task$/i })).toBeInTheDocument();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /saved drafts are unavailable right now/i,
    );
    expect(
      screen.getByRole("button", { name: /retry loading drafts/i }),
    ).toBeInTheDocument();
  });

  it("retries draft loading from home entry hint and opens draft picker when available", async () => {
    const user = userEvent.setup();
    let draftAttempts = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        draftAttempts += 1;
        if (draftAttempts === 1) {
          return new Response(
            JSON.stringify({ error: "drafts unavailable" }),
            { status: 500, headers: { "Content-Type": "application/json" } },
          );
        }
        return Response.json({
          drafts: [
            {
              id: "d1",
              name: "Recovered draft",
              created_at: "2026-04-07T10:00:00Z",
              updated_at: "2026-04-07T10:05:00Z",
            },
          ],
        });
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
    await screen.findByText("No tasks yet");
    await user.click(screen.getByRole("button", { name: /^new task$/i }));
    await user.click(screen.getByRole("button", { name: /retry loading drafts/i }));

    expect(
      await screen.findByRole("heading", { name: /resume a draft or start fresh/i }),
    ).toBeInTheDocument();
  });

  it("shows resume error on drafts page when opening a draft fails", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        return Response.json({
          drafts: [
            {
              id: "d1",
              name: "Broken draft",
              created_at: "2026-04-07T10:00:00Z",
              updated_at: "2026-04-07T10:05:00Z",
            },
          ],
        });
      }
      if (url === "/task-drafts/d1") {
        return new Response(
          JSON.stringify({ error: "resume failed" }),
          { status: 500, headers: { "Content-Type": "application/json" } },
        );
      }
      if (url.startsWith("/repo/")) {
        return new Response(
          JSON.stringify({ error: "repo not configured" }),
          { status: 503 },
        );
      }
      return new Response("not found", { status: 404 });
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={["/drafts"]}>
          <App />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await screen.findByRole("heading", { name: /^task drafts$/i });
    await user.click(
      await screen.findByRole("button", {
        name: /open draft broken draft in create form/i,
      }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(/resume failed/i);
  });

  it("shows delete error on drafts page when deleting a draft fails", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        return Response.json({
          drafts: [
            {
              id: "d1",
              name: "Delete me",
              created_at: "2026-04-07T10:00:00Z",
              updated_at: "2026-04-07T10:05:00Z",
            },
          ],
        });
      }
      if (url === "/task-drafts/d1" && init?.method === "DELETE") {
        return new Response(
          JSON.stringify({ error: "delete failed" }),
          { status: 500, headers: { "Content-Type": "application/json" } },
        );
      }
      if (url.startsWith("/repo/")) {
        return new Response(
          JSON.stringify({ error: "repo not configured" }),
          { status: 503 },
        );
      }
      return new Response("not found", { status: 404 });
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={["/drafts"]}>
          <App />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await screen.findByRole("heading", { name: /^task drafts$/i });
    await user.click(await screen.findByRole("button", { name: /^delete$/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent(/delete failed/i);
  });

  it("does not autosave untouched fresh drafts", async () => {
    const user = userEvent.setup();
    const draftSaves: string[] = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        return Response.json({ drafts: [] });
      }
      if (url === "/task-drafts" && init?.method === "POST") {
        draftSaves.push(String(init.body ?? ""));
        return new Response(
          JSON.stringify({ id: "d1", name: "Untitled draft" }),
          { status: 201, headers: { "Content-Type": "application/json" } },
        );
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
    await screen.findByText("No tasks yet");
    await user.click(screen.getByRole("button", { name: /^new task$/i }));
    await screen.findByRole("dialog", { name: /^new task$/i });

    await new Promise((resolve) => setTimeout(resolve, 1200));
    await waitFor(() => {
      expect(draftSaves).toHaveLength(0);
    });
  });

  it("shows delete loading state only for clicked draft row", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({ tasks: [], limit: 200, offset: 0 });
      }
      if (url.startsWith("/task-drafts?")) {
        return Response.json({
          drafts: [
            {
              id: "d1",
              name: "First draft",
              created_at: "2026-04-07T10:00:00Z",
              updated_at: "2026-04-07T10:05:00Z",
            },
            {
              id: "d2",
              name: "Second draft",
              created_at: "2026-04-07T11:00:00Z",
              updated_at: "2026-04-07T11:05:00Z",
            },
          ],
        });
      }
      if (url === "/task-drafts/d1" && init?.method === "DELETE") {
        return new Promise<Response>(() => {});
      }
      if (url.startsWith("/repo/")) {
        return new Response(
          JSON.stringify({ error: "repo not configured" }),
          { status: 503 },
        );
      }
      return new Response("not found", { status: 404 });
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={["/drafts"]}>
          <App />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await screen.findByRole("heading", { name: /^task drafts$/i });
    const firstResume = await screen.findByRole("button", {
      name: /open draft first draft in create form/i,
    });
    const secondResume = await screen.findByRole("button", {
      name: /open draft second draft in create form/i,
    });
    const firstRow = firstResume.parentElement as HTMLElement;
    const secondRow = secondResume.parentElement as HTMLElement;

    await user.click(within(firstRow).getByRole("button", { name: /^delete$/i }));

    expect(within(firstRow).getByRole("button", { name: /^deleting…$/i })).toBeInTheDocument();
    expect(within(secondRow).getByRole("button", { name: /^delete$/i })).toBeInTheDocument();
  });

  it("creates a top-level task with checklist criteria added after create", async () => {
    const user = userEvent.setup();
    let created = false;
    const checklistPosts: string[] = [];

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
              title: "With criteria",
              initial_prompt: "",
              status: "ready",
              priority: "medium",
              checklist_inherit: false,
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
            title: "With criteria",
            initial_prompt: "",
            status: "ready",
            priority: "medium",
            checklist_inherit: false,
          }),
          { status: 201, headers: { "Content-Type": "application/json" } },
        );
      }
      if (url === "/tasks/t1/checklist/items" && init?.method === "POST") {
        const body = init?.body != null ? String(init.body) : "";
        checklistPosts.push(body);
        return new Response(null, { status: 204 });
      }
      return new Response("not found", { status: 404 });
    });

    renderApp();
    await screen.findByText("No tasks yet");

    const dialog = await openNewTaskModal(user);
    await user.type(within(dialog).getByLabelText(/^title$/i), "With criteria");
    await choosePriorityInDialog(user, dialog);
    await user.click(
      within(dialog).getByRole("button", { name: /new criterion/i }),
    );
    const criterionDialog = await screen.findByRole("dialog", {
      name: /new criterion/i,
    });
    await user.type(
      within(criterionDialog).getByLabelText(/^criterion$/i),
      "Tests pass",
    );
    await user.click(
      within(criterionDialog).getByRole("button", { name: /^add criterion$/i }),
    );

    await user.click(
      within(dialog).getByRole("button", { name: /^create$/i }),
    );

    expect(
      await screen.findByRole("link", { name: /with criteria/i }),
    ).toBeInTheDocument();
    expect(checklistPosts).toHaveLength(1);
    expect(checklistPosts[0]).toContain("Tests pass");
  });

  it("creates a top-level task using edited checklist criterion text", async () => {
    const user = userEvent.setup();
    let created = false;
    const checklistPosts: string[] = [];

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
              title: "With edited criteria",
              initial_prompt: "",
              status: "ready",
              priority: "medium",
              checklist_inherit: false,
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
            title: "With edited criteria",
            initial_prompt: "",
            status: "ready",
            priority: "medium",
            checklist_inherit: false,
          }),
          { status: 201, headers: { "Content-Type": "application/json" } },
        );
      }
      if (url === "/tasks/t1/checklist/items" && init?.method === "POST") {
        const body = init?.body != null ? String(init.body) : "";
        checklistPosts.push(body);
        return new Response(null, { status: 204 });
      }
      return new Response("not found", { status: 404 });
    });

    renderApp();
    await screen.findByText("No tasks yet");

    const dialog = await openNewTaskModal(user);
    await user.type(
      within(dialog).getByLabelText(/^title$/i),
      "With edited criteria",
    );
    await choosePriorityInDialog(user, dialog);
    await user.click(
      within(dialog).getByRole("button", { name: /new criterion/i }),
    );
    const addCriterionDialog = await screen.findByRole("dialog", {
      name: /new criterion/i,
    });
    await user.type(
      within(addCriterionDialog).getByLabelText(/^criterion$/i),
      "Old wording",
    );
    await user.click(
      within(addCriterionDialog).getByRole("button", { name: /^add criterion$/i }),
    );

    await user.click(within(dialog).getByRole("button", { name: /^edit$/i }));
    const editCriterionDialog = await screen.findByRole("dialog", {
      name: /edit criterion/i,
    });
    const criterionInput = within(editCriterionDialog).getByLabelText(
      /^criterion$/i,
    );
    await user.clear(criterionInput);
    await user.type(criterionInput, "Updated wording");
    await user.click(
      within(editCriterionDialog).getByRole("button", { name: /^save changes$/i }),
    );

    await user.click(
      within(dialog).getByRole("button", { name: /^create$/i }),
    );

    expect(
      await screen.findByRole("link", { name: /with edited criteria/i }),
    ).toBeInTheDocument();
    expect(checklistPosts).toHaveLength(1);
    expect(checklistPosts[0]).toContain("Updated wording");
  });

  it("does not expose a parent picker on the home create-task modal", async () => {
    // Subtasks are now created from inside the parent task page
    // (`SubtaskCreateModal`), not from the home modal. The home
    // modal is exclusively for top-level tasks.
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({
          tasks: [
            {
              id: "p1",
              title: "Check parent",
              initial_prompt: "",
              status: "ready",
              priority: "medium",
              checklist_inherit: false,
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
      return new Response("not found", { status: 404 });
    });

    renderApp();
    expect(await screen.findByText("Check parent")).toBeInTheDocument();

    const dialog = await openNewTaskModal(user);
    expect(
      within(dialog).getByRole("heading", { name: /^new task$/i }),
    ).toBeInTheDocument();
    expect(
      within(dialog).queryByRole("combobox", { name: /^parent task$/i }),
    ).not.toBeInTheDocument();
    expect(
      within(dialog).queryByText(/inherit parent's checklist criteria/i),
    ).not.toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: /^create$/i }),
    ).toBeInTheDocument();
    expect(
      within(dialog).queryByRole("button", { name: /^add subtask$/i }),
    ).not.toBeInTheDocument();
  });

  it("creates a task with subtasks after the parent task", async () => {
    const user = userEvent.setup();
    let postCount = 0;
    const taskPosts: Array<Record<string, unknown>> = [];

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        if (postCount === 0) {
          return Response.json({ tasks: [], limit: 200, offset: 0 });
        }
        return Response.json({
          tasks: [
            {
              id: "parent",
              title: "Epic",
              initial_prompt: "",
              status: "ready",
              priority: "medium",
              checklist_inherit: false,
              children: [
                {
                  id: "c1",
                  title: "Step one",
                  initial_prompt: "",
                  status: "ready",
                  priority: "medium",
                  checklist_inherit: false,
                },
                {
                  id: "c2",
                  title: "Step two",
                  initial_prompt: "",
                  status: "ready",
                  priority: "medium",
                  checklist_inherit: false,
                },
              ],
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
        postCount++;
        const body =
          init?.body != null && typeof init.body === "string"
            ? (JSON.parse(init.body) as Record<string, unknown>)
            : {};
        taskPosts.push(body);
        if (postCount === 1) {
          return new Response(
            JSON.stringify({
              id: "parent",
              title: body.title,
              initial_prompt: body.initial_prompt ?? "",
              status: body.status ?? "ready",
              priority: body.priority ?? "medium",
              checklist_inherit: false,
            }),
            { status: 201, headers: { "Content-Type": "application/json" } },
          );
        }
        const id = postCount === 2 ? "c1" : "c2";
        return new Response(
          JSON.stringify({
            id,
            title: body.title,
            initial_prompt: "",
            status: body.status ?? "ready",
            priority: body.priority ?? "medium",
            checklist_inherit: false,
            parent_id: body.parent_id,
          }),
          { status: 201, headers: { "Content-Type": "application/json" } },
        );
      }
      return new Response("not found", { status: 404 });
    });

    renderApp();
    await screen.findByText("No tasks yet");

    const outer = await openNewTaskModal(user);
    await user.type(within(outer).getByLabelText(/^title$/i), "Epic");
    await choosePriorityInDialog(user, outer);

    await user.click(
      within(outer).getByRole("button", { name: /open form to add a subtask/i }),
    );
    let dialogs = screen.getAllByRole("dialog");
    expect(dialogs.length).toBe(2);
    let nested = dialogs[1];
    await user.type(within(nested).getByLabelText(/^title$/i), "Step one");
    await choosePriorityInDialog(user, nested);
    await user.click(
      within(nested).getByRole("button", { name: /^add subtask$/i }),
    );
    await waitFor(() => {
      expect(screen.getAllByRole("dialog")).toHaveLength(1);
    });

    await user.click(
      within(outer).getByRole("button", { name: /open form to add a subtask/i }),
    );
    dialogs = screen.getAllByRole("dialog");
    expect(dialogs.length).toBe(2);
    nested = dialogs[1];
    await user.type(within(nested).getByLabelText(/^title$/i), "Step two");
    await choosePriorityInDialog(user, nested);
    await user.click(
      within(nested).getByRole("button", { name: /^add subtask$/i }),
    );
    await waitFor(() => {
      expect(screen.getAllByRole("dialog")).toHaveLength(1);
    });

    await user.click(
      within(outer).getByRole("button", { name: /^create$/i }),
    );

    expect(
      await screen.findByRole("link", { name: /epic/i }),
    ).toBeInTheDocument();
    expect(taskPosts).toHaveLength(3);
    expect(taskPosts[0].title).toBe("Epic");
    expect(taskPosts[0].parent_id).toBeUndefined();
    const childPosts = taskPosts.slice(1) as Array<{
      parent_id?: unknown;
      title?: unknown;
    }>;
    expect(childPosts).toHaveLength(2);
    for (const child of childPosts) {
      expect(child.parent_id).toBe("parent");
    }
    expect(childPosts.map((p) => p.title).sort()).toEqual([
      "Step one",
      "Step two",
    ]);
  });

  it("posts a top-level task with no parent_id from the home create modal", async () => {
    // The home modal is now top-level-only; verifies no parent_id
    // is ever attached even when parent tasks exist in the list.
    const user = userEvent.setup();
    let postBody: Record<string, unknown> | null = null;

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        return Response.json({
          tasks: [
            {
              id: "parent",
              title: "Parent task",
              initial_prompt: "",
              status: "ready",
              priority: "medium",
              checklist_inherit: false,
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
        postBody =
          init?.body != null && typeof init.body === "string"
            ? JSON.parse(init.body)
            : {};
        return new Response(
          JSON.stringify({
            id: "new",
            title: "Standalone task",
            initial_prompt: "",
            status: "ready",
            priority: "medium",
            checklist_inherit: false,
          }),
          { status: 201, headers: { "Content-Type": "application/json" } },
        );
      }
      return new Response("not found", { status: 404 });
    });

    renderApp();
    expect(await screen.findByText("Parent task")).toBeInTheDocument();

    const dialog = await openNewTaskModal(user);
    await user.type(
      within(dialog).getByLabelText(/^title$/i),
      "Standalone task",
    );
    await choosePriorityInDialog(user, dialog);
    await user.click(
      within(dialog).getByRole("button", { name: /^create$/i }),
    );

    await waitFor(() => {
      expect(postBody).not.toBeNull();
    });
    const posted = postBody as unknown as {
      parent_id?: unknown;
      checklist_inherit?: unknown;
      title?: unknown;
    };
    expect(posted.title).toBe("Standalone task");
    expect(posted.parent_id).toBeUndefined();
    expect(posted.checklist_inherit).toBeUndefined();
  });
});
