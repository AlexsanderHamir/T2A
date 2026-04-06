import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { useTasksApp } from "../hooks/useTasksApp";
import { stubEventSource } from "../../test/browserMocks";
import { requestUrl } from "../../test/requestUrl";
import { ROUTER_FUTURE_FLAGS } from "../../lib/routerFutureFlags";
import { DEFAULT_DOCUMENT_TITLE } from "../../shared/useDocumentTitle";
import { TaskDetailPage } from "./TaskDetailPage";

const { mockNavigate } = vi.hoisted(() => ({ mockNavigate: vi.fn() }));

vi.mock("react-router-dom", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router-dom")>();
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

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
      <MemoryRouter
        future={ROUTER_FUTURE_FLAGS}
        initialEntries={[initialPath]}
      >
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
    mockNavigate.mockClear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("shows a loading skeleton while the task query is pending", () => {
    vi.spyOn(globalThis, "fetch").mockImplementation((input) => {
      const url = requestUrl(input);
      if (url === "/tasks/t1") {
        return new Promise<Response>(() => {
          /* never resolves — keep task detail pending */
        });
      }
      return Promise.resolve(new Response("not found", { status: 404 }));
    });
    renderDetail("/tasks/t1", mockApp());
    expect(
      screen.getByRole("status", { name: /loading task/i }),
    ).toBeInTheDocument();
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
    expect(document.title).toBe(`Testing · ${DEFAULT_DOCUMENT_TITLE}`);
    const stance = await screen.findByText("Informational");
    expect(stance).toHaveAttribute("data-stance", "informational");
    expect(await screen.findByText(/no updates yet/i)).toBeInTheDocument();

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

  it("sanitizes unsafe HTML from initial prompt before rendering", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/txss") {
        return Response.json({
          id: "txss",
          title: "Unsafe prompt",
          initial_prompt:
            '<p>Safe text</p><img src=x onerror="window.__xss = 1" /><script>window.__xss_script = 1</script><a href="javascript:alert(1)">bad</a>',
          status: "ready",
          priority: "medium",
          checklist_inherit: false,
        });
      }
      if (url === "/tasks/txss/checklist") {
        return Response.json({ items: [] });
      }
      if (url.startsWith("/tasks/txss/events")) {
        return Response.json({
          task_id: "txss",
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

    renderDetail("/tasks/txss", mockApp());
    expect(
      await screen.findByRole("heading", { name: /^unsafe prompt$/i }),
    ).toBeInTheDocument();

    const promptBody = document.querySelector(
      ".task-detail-prompt-body",
    ) as HTMLElement | null;
    expect(promptBody).not.toBeNull();
    expect(promptBody!.innerHTML).not.toContain("<script");
    expect(promptBody!.innerHTML).not.toContain("onerror=");
    expect(promptBody!.innerHTML).not.toContain("javascript:");
    expect(promptBody).toHaveTextContent(/safe text/i);
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

  it("navigates to parent task after successful delete when parent_id is set", () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/sub1") {
        return Response.json({
          id: "sub1",
          title: "Sub",
          initial_prompt: "",
          status: "ready",
          priority: "medium",
          parent_id: "par1",
          checklist_inherit: false,
        });
      }
      if (url === "/tasks/sub1/checklist") {
        return Response.json({ items: [] });
      }
      if (url.startsWith("/tasks/sub1/events")) {
        return Response.json({
          task_id: "sub1",
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

    const app = {
      ...mockApp(),
      deleteMutation: {
        isSuccess: true,
        variables: { id: "sub1", parent_id: "par1" },
      },
    } as unknown as ReturnType<typeof useTasksApp>;

    renderDetail("/tasks/sub1", app);

    expect(mockNavigate).toHaveBeenCalledWith("/tasks/par1", { replace: true });
  });

  it("navigates home after successful delete when task has no parent", () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/root1") {
        return Response.json({
          id: "root1",
          title: "Root",
          initial_prompt: "",
          status: "ready",
          priority: "medium",
          checklist_inherit: false,
        });
      }
      if (url === "/tasks/root1/checklist") {
        return Response.json({ items: [] });
      }
      if (url.startsWith("/tasks/root1/events")) {
        return Response.json({
          task_id: "root1",
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

    const app = {
      ...mockApp(),
      deleteMutation: {
        isSuccess: true,
        variables: { id: "root1" },
      },
    } as unknown as ReturnType<typeof useTasksApp>;

    renderDetail("/tasks/root1", app);

    expect(mockNavigate).toHaveBeenCalledWith("/", { replace: true });
  });

  it("edits a checklist criterion via PATCH text", async () => {
    const user = userEvent.setup();
    let patchBody: string | null = null;
    let checklistText = "Before";

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = requestUrl(input);
      const method = init?.method ?? "GET";
      if (url === "/tasks/te") {
        return Response.json({
          id: "te",
          title: "Edit checklist",
          initial_prompt: "",
          status: "ready",
          priority: "medium",
          checklist_inherit: false,
        });
      }
      if (url === "/tasks/te/checklist") {
        return Response.json({
          items: [
            {
              id: "item-1",
              sort_order: 0,
              text: checklistText,
              done: false,
            },
          ],
        });
      }
      if (url === "/tasks/te/checklist/items/item-1" && method === "PATCH") {
        patchBody = (init?.body as string) ?? null;
        checklistText = "After";
        return Response.json({
          items: [
            {
              id: "item-1",
              sort_order: 0,
              text: "After",
              done: false,
            },
          ],
        });
      }
      if (url.startsWith("/tasks/te/events")) {
        return Response.json({
          task_id: "te",
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

    renderDetail("/tasks/te", mockApp());

    expect(
      await screen.findByRole("heading", { name: /^edit checklist$/i }),
    ).toBeInTheDocument();

    expect(await screen.findByText("Before")).toBeInTheDocument();

    const checklistSection = document.querySelector("#task-detail-checklist");
    expect(checklistSection).not.toBeNull();
    await user.click(
      await within(checklistSection as HTMLElement).findByRole("button", {
        name: /^edit$/i,
      }),
    );

    const dialog = await screen.findByRole("dialog");
    const input = within(dialog).getByLabelText(/^criterion$/i);
    await user.clear(input);
    await user.type(input, "After");

    await user.click(
      within(dialog).getByRole("button", { name: /^save changes$/i }),
    );

    expect(patchBody).toBe(JSON.stringify({ text: "After" }));
    expect(await screen.findByText("After")).toBeInTheDocument();
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
        expect(body.priority).toBe("high");
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
    await user.click(
      within(dialog).getByRole("combobox", { name: /^priority$/i }),
    );
    await user.click(screen.getByRole("option", { name: /^high$/i }));
    await user.click(
      within(dialog).getByRole("button", { name: /new criterion/i }),
    );
    const criterionDialog = await screen.findByRole("dialog", {
      name: /new criterion/i,
    });
    await user.type(
      within(criterionDialog).getByLabelText(/^criterion$/i),
      "Criterion A",
    );
    await user.click(
      within(criterionDialog).getByRole("button", { name: /^add criterion$/i }),
    );

    await user.click(
      within(dialog).getByRole("button", { name: /^add subtask$/i }),
    );

    expect(await screen.findByText("Child")).toBeInTheDocument();
    const childLink = screen.getByRole("link", { name: "Child" });
    const subtaskRow = childLink.closest(
      ".task-subtasks-item-row",
    ) as HTMLElement | null;
    expect(subtaskRow).not.toBeNull();
    expect(within(subtaskRow!).getByText("high")).toBeInTheDocument();
    expect(within(subtaskRow!).getByText("ready")).toBeInTheDocument();
    expect(checklistPosts).toHaveLength(1);
    expect(checklistPosts[0]).toContain("Criterion A");
  });
});
