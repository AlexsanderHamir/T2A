import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { useTasksApp } from "../hooks/useTasksApp";
import { stubEventSource } from "../../test/browserMocks";
import { requestUrl } from "../../test/requestUrl";
import { TaskDetailPage } from "./TaskDetailPage";

function mockApp(): ReturnType<typeof useTasksApp> {
  return {
    deleteMutation: { isSuccess: false, variables: undefined },
    openEdit: vi.fn(),
    requestDelete: vi.fn(),
    saving: false,
  } as unknown as ReturnType<typeof useTasksApp>;
}

function renderDetail(
  initialPath: string,
  app: ReturnType<typeof useTasksApp>,
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route
            path="/tasks/:taskId"
            element={<TaskDetailPage app={app} />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("TaskDetailPage", () => {
  beforeEach(() => {
    stubEventSource();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("collapses initial prompt by default and expands on demand", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/t1") {
        return Response.json({
          id: "t1",
          title: "Testing",
          initial_prompt: "<p>Secret long body text</p>",
          status: "ready",
          priority: "critical",
          checklist_inherit: false,
        });
      }
      if (url === "/tasks/t1/checklist") {
        return Response.json({ items: [] });
      }
      if (url.startsWith("/tasks/t1/events")) {
        return Response.json({
          task_id: "t1",
          events: [],
          limit: 20,
          total: 0,
          has_more_newer: false,
          has_more_older: false,
          approval_pending: false,
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderDetail("/tasks/t1", mockApp());

    expect(await screen.findByRole("heading", { name: /^testing$/i })).toBeInTheDocument();
    const stance = await screen.findByText("Informational");
    expect(stance).toHaveAttribute("data-stance", "informational");
    expect(await screen.findByText(/no audit events yet/i)).toBeInTheDocument();

    const details = document.querySelector(".task-detail-prompt-details");
    expect(details).not.toBeNull();
    expect(details).not.toHaveAttribute("open");

    expect(
      await screen.findByText(/show full initial prompt/i),
    ).toBeInTheDocument();

    await user.click(screen.getByText(/show full initial prompt/i));
    expect(details).toHaveAttribute("open");
    expect(screen.getByText("Secret long body text")).toBeVisible();
    expect(screen.getByText(/hide initial prompt/i)).toBeInTheDocument();
  });

  it("shows an em dash when there is no visible initial prompt", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/t2") {
        return Response.json({
          id: "t2",
          title: "Empty prompt",
          initial_prompt: "",
          status: "ready",
          priority: "medium",
          checklist_inherit: false,
        });
      }
      if (url === "/tasks/t2/checklist") {
        return Response.json({ items: [] });
      }
      if (url.startsWith("/tasks/t2/events")) {
        return Response.json({
          task_id: "t2",
          events: [],
          limit: 20,
          total: 0,
          has_more_newer: false,
          has_more_older: false,
          approval_pending: false,
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderDetail("/tasks/t2", mockApp());

    expect(
      await screen.findByRole("heading", { name: /^empty prompt$/i }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/show full initial prompt/i)).not.toBeInTheDocument();
    const empty = screen.getByText("—");
    expect(empty).toBeInTheDocument();
    expect(empty).toHaveClass("task-detail-prompt-empty");
  });

  it("shows status stance when the task status needs user input", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/tb") {
        return Response.json({
          id: "tb",
          title: "Blocked task",
          initial_prompt: "",
          status: "blocked",
          priority: "medium",
          checklist_inherit: false,
        });
      }
      if (url === "/tasks/tb/checklist") {
        return Response.json({ items: [] });
      }
      if (url.startsWith("/tasks/tb/events")) {
        return Response.json({
          task_id: "tb",
          events: [],
          limit: 20,
          total: 0,
          has_more_newer: false,
          has_more_older: false,
          approval_pending: false,
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderDetail("/tasks/tb", mockApp());

    expect(
      await screen.findByRole("heading", { name: /^blocked task$/i }),
    ).toBeInTheDocument();
    const stance = await screen.findByText("Agent needs input");
    expect(stance).toHaveAttribute("data-stance", "needs-user");
  });

  it("shows done criteria as read-only with progress counts", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/tc") {
        return Response.json({
          id: "tc",
          title: "Checklist task",
          initial_prompt: "",
          status: "ready",
          priority: "medium",
          checklist_inherit: false,
        });
      }
      if (url === "/tasks/tc/checklist") {
        return Response.json({
          items: [
            {
              id: "i1",
              sort_order: 0,
              text: "First",
              done: true,
            },
            {
              id: "i2",
              sort_order: 1,
              text: "Second",
              done: false,
            },
          ],
        });
      }
      if (url.startsWith("/tasks/tc/events")) {
        return Response.json({
          task_id: "tc",
          events: [],
          limit: 20,
          total: 0,
          has_more_newer: false,
          has_more_older: false,
          approval_pending: false,
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderDetail("/tasks/tc", mockApp());

    expect(
      await screen.findByRole("heading", { name: /^checklist task$/i }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("status", {
        name: /checklist progress: 1 of 2 requirements satisfied/i,
      }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(screen.getByText("First")).toBeInTheDocument();
    expect(screen.getByText("Second")).toBeInTheDocument();
  });

  it("lists updates newest first by seq", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/t3") {
        return Response.json({
          id: "t3",
          title: "Timeline order",
          initial_prompt: "",
          status: "ready",
          priority: "medium",
          checklist_inherit: false,
        });
      }
      if (url === "/tasks/t3/checklist") {
        return Response.json({ items: [] });
      }
      if (url.startsWith("/tasks/t3/events")) {
        return Response.json({
          task_id: "t3",
          limit: 20,
          total: 2,
          range_start: 1,
          range_end: 2,
          has_more_newer: false,
          has_more_older: false,
          approval_pending: false,
          events: [
            {
              seq: 2,
              at: "2026-01-02T12:00:00.000Z",
              type: "sync_ping",
              by: "user",
              data: {},
            },
            {
              seq: 1,
              at: "2026-01-01T12:00:00.000Z",
              type: "task_created",
              by: "user",
              data: {},
            },
          ],
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderDetail("/tasks/t3", mockApp());

    expect(
      await screen.findByRole("heading", { name: /^timeline order$/i }),
    ).toBeInTheDocument();

    const timeline = await screen.findByRole("list", { name: /^updates$/i });
    const items = within(timeline).getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent(/sync_ping/i);
    expect(items[1]).toHaveTextContent(/task_created/i);
    expect(items[0].querySelector("code.task-timeline-type-pill")).toHaveAttribute(
      "aria-label",
      expect.stringMatching(/live sync check/i),
    );
  });

  it("creates a subtask with checklist items after POST /tasks", async () => {
    const user = userEvent.setup();
    const checklistPosts: string[] = [];
    let subtaskCreated = false;

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      const method = init?.method ?? "GET";
      if (url === "/tasks/parent" && method === "GET") {
        return Response.json({
          id: "parent",
          title: "Parent",
          initial_prompt: "",
          status: "ready",
          priority: "medium",
          checklist_inherit: false,
          children: subtaskCreated
            ? [
                {
                  id: "child",
                  title: "Child",
                  initial_prompt: "<p></p>",
                  status: "ready",
                  priority: "high",
                  checklist_inherit: false,
                },
              ]
            : [],
        });
      }
      if (url === "/tasks/parent/checklist") {
        return Response.json({ items: [] });
      }
      if (url.startsWith("/tasks/parent/events")) {
        return Response.json({
          task_id: "parent",
          events: [],
          limit: 20,
          total: 0,
          has_more_newer: false,
          has_more_older: false,
          approval_pending: false,
        });
      }
      if (url === "/tasks" && method === "POST") {
        const body =
          init?.body != null && typeof init.body === "string"
            ? JSON.parse(init.body)
            : {};
        expect(body.parent_id).toBe("parent");
        expect(body.title).toBe("Child");
        subtaskCreated = true;
        return new Response(
          JSON.stringify({
            id: "child",
            title: "Child",
            initial_prompt: "<p></p>",
            status: "ready",
            priority: "high",
            checklist_inherit: false,
          }),
          { status: 201, headers: { "Content-Type": "application/json" } },
        );
      }
      if (url === "/tasks/child/checklist/items" && method === "POST") {
        checklistPosts.push(
          init?.body != null ? String(init.body) : "",
        );
        return new Response(null, { status: 204 });
      }
      return new Response("not found", { status: 404 });
    });

    renderDetail("/tasks/parent", mockApp());

    expect(await screen.findByRole("heading", { name: /^parent$/i })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /^add subtask$/i }));

    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText(/^title$/i), "Child");
    await user.type(
      within(dialog).getByPlaceholderText(/describe what must be true/i),
      "Criterion A",
    );
    await user.click(
      within(dialog).getByRole("button", { name: /add checklist criterion/i }),
    );

    await user.click(
      within(dialog).getByRole("button", { name: /^add subtask$/i }),
    );

    expect(await screen.findByText("Child")).toBeInTheDocument();
    expect(checklistPosts).toHaveLength(1);
    expect(checklistPosts[0]).toContain("Criterion A");
  });
});
