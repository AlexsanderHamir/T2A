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

  it("exposes Tasks title as home link with aria-current on /", async () => {
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
    await screen.findByRole("heading", { name: /^tasks$/i });
    const titleLink = screen.getByRole("link", { name: /^tasks$/i });
    expect(titleLink).toHaveAttribute("href", "/");
    expect(titleLink).toHaveAttribute("aria-current", "page");
  });

  it("navigates home when Tasks title is used from a task route", async () => {
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
    const titleLink = screen.getByRole("link", { name: /^tasks$/i });
    expect(titleLink).not.toHaveAttribute("aria-current");

    await user.click(titleLink);
    expect(await screen.findByText("No tasks yet")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /^tasks$/i })).toHaveAttribute(
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
      await screen.findByRole("heading", { name: /^tasks$/i }),
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

  it("creates a subtask from home with checklist criteria added after create", async () => {
    const user = userEvent.setup();
    let created = false;
    const checklistPosts: string[] = [];

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        if (!created) {
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
        return Response.json({
          tasks: [
            {
              id: "p1",
              title: "Check parent",
              initial_prompt: "",
              status: "ready",
              priority: "medium",
              checklist_inherit: false,
              children: [
                {
                  id: "t1",
                  title: "With criteria",
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
        created = true;
        return new Response(
          JSON.stringify({
            id: "t1",
            title: "With criteria",
            initial_prompt: "",
            status: "ready",
            priority: "medium",
            checklist_inherit: false,
            parent_id: "p1",
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
    expect(await screen.findByText("Check parent")).toBeInTheDocument();

    const dialog = await openNewTaskModal(user);
    const parentSelect = within(dialog).getByLabelText(/^parent task$/i);
    await waitFor(() => {
      expect(
        within(parentSelect).getByRole("option", { name: "Check parent" }),
      ).toBeInTheDocument();
    });
    await user.selectOptions(parentSelect, "p1");
    expect(
      await within(dialog).findByRole("heading", { name: /^new subtask$/i }),
    ).toBeInTheDocument();

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
      within(dialog).getByRole("button", { name: /^add subtask$/i }),
    );

    expect(
      await screen.findByRole("link", { name: /check parent/i }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /with criteria/i })).toBeNull();
    expect(checklistPosts).toHaveLength(1);
    expect(checklistPosts[0]).toContain("Tests pass");
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
    expect(taskPosts[1].parent_id).toBe("parent");
    expect(taskPosts[1].title).toBe("Step one");
    expect(taskPosts[2].parent_id).toBe("parent");
    expect(taskPosts[2].title).toBe("Step two");
  });

  it("creates a subtask from home when parent task is selected", async () => {
    const user = userEvent.setup();
    let created = false;
    let postBody: Record<string, unknown> | null = null;

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      if (url.startsWith("/tasks?")) {
        if (!created) {
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
        return Response.json({
          tasks: [
            {
              id: "parent",
              title: "Parent task",
              initial_prompt: "",
              status: "ready",
              priority: "medium",
              checklist_inherit: false,
              children: [
                {
                  id: "child",
                  title: "Child sub",
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
        created = true;
        postBody =
          init?.body != null && typeof init.body === "string"
            ? JSON.parse(init.body)
            : {};
        return new Response(
          JSON.stringify({
            id: "child",
            title: "Child sub",
            initial_prompt: "",
            status: "ready",
            priority: "medium",
            checklist_inherit: false,
            parent_id: "parent",
          }),
          { status: 201, headers: { "Content-Type": "application/json" } },
        );
      }
      return new Response("not found", { status: 404 });
    });

    renderApp();
    expect(await screen.findByText("Parent task")).toBeInTheDocument();

    const dialog = await openNewTaskModal(user);
    const parentSelect = within(dialog).getByLabelText(/^parent task$/i);
    await waitFor(() => {
      expect(
        within(parentSelect).getByRole("option", { name: "Parent task" }),
      ).toBeInTheDocument();
    });
    await user.selectOptions(parentSelect, "parent");
    expect(
      await within(dialog).findByRole("heading", { name: /^new subtask$/i }),
    ).toBeInTheDocument();
    await user.type(within(dialog).getByLabelText(/^title$/i), "Child sub");
    await choosePriorityInDialog(user, dialog);
    await user.click(
      within(dialog).getByRole("button", { name: /^add subtask$/i }),
    );

    expect(
      await screen.findByRole("link", { name: /parent task/i }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /child sub/i })).toBeNull();
    expect(postBody).not.toBeNull();
    const posted = postBody as unknown as {
      parent_id?: unknown;
      title?: unknown;
    };
    expect(posted.parent_id).toBe("parent");
    expect(posted.title).toBe("Child sub");
  });
});
