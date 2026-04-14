import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
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

function appWithDeleteSuccess(
  variables: { id: string; parent_id?: string },
): ReturnType<typeof useTasksApp> {
  return {
    ...mockApp(),
    deleteMutation: {
      isSuccess: true,
      variables,
    },
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

function emptyEventsPayload(taskId: string) {
  return {
    task_id: taskId,
    events: [],
    limit: 20,
    total: 0,
    has_more_newer: false,
    has_more_older: false,
    approval_pending: false,
  };
}

type MockTaskDetailData = {
  id: string;
  title: string;
  initial_prompt: string;
  status: string;
  priority: string;
  checklist_inherit: boolean;
  parent_id?: string;
};

function taskDetail(
  id: string,
  title: string,
  overrides: Partial<MockTaskDetailData> = {},
): MockTaskDetailData {
  return {
    id,
    title,
    initial_prompt: "",
    status: "ready",
    priority: "medium",
    checklist_inherit: false,
    ...overrides,
  };
}

function mockTaskDetailFetch(
  task: MockTaskDetailData,
  checklistItems: unknown[] = [],
) {
  return vi
    .spyOn(globalThis, "fetch")
    .mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === `/tasks/${task.id}`) {
        return Response.json(task);
      }
      if (url === `/tasks/${task.id}/checklist`) {
        return Response.json({ items: checklistItems });
      }
      if (url.startsWith(`/tasks/${task.id}/events`)) {
        return Response.json(emptyEventsPayload(task.id));
      }
      return new Response("not found", { status: 404 });
    });
}

function mockTaskDetailFetchWithChecklistPatch(
  task: MockTaskDetailData,
  checklistItemId: string,
  initialText: string,
  nextText: string,
) {
  let patchBody: string | null = null;
  let checklistText = initialText;
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const url = requestUrl(input);
    const method = init?.method ?? "GET";
    if (url === `/tasks/${task.id}`) {
      return Response.json(task);
    }
    if (url === `/tasks/${task.id}/checklist`) {
      return Response.json({
        items: [
          {
            id: checklistItemId,
            sort_order: 0,
            text: checklistText,
            done: false,
          },
        ],
      });
    }
    if (
      url === `/tasks/${task.id}/checklist/items/${checklistItemId}` &&
      method === "PATCH"
    ) {
      patchBody = (init?.body as string) ?? null;
      checklistText = nextText;
      return Response.json({
        items: [
          {
            id: checklistItemId,
            sort_order: 0,
            text: nextText,
            done: false,
          },
        ],
      });
    }
    if (url.startsWith(`/tasks/${task.id}/events`)) {
      return Response.json(emptyEventsPayload(task.id));
    }
    return new Response("not found", { status: 404 });
  });
  return {
    getPatchBody: () => patchBody,
  };
}

function mockTaskDetailFetchForSubtaskCreate(taskId: string) {
  const checklistPosts: string[] = [];
  let subtaskCreated = false;
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const url = requestUrl(input);
    const method = init?.method ?? "GET";
    if (url === `/tasks/${taskId}` && method === "GET") {
      return Response.json({
        ...taskDetail(taskId, "Parent"),
        children: subtaskCreated
          ? [
              {
                ...taskDetail("child", "Child", {
                  priority: "high",
                  initial_prompt: "<p></p>",
                }),
              },
            ]
          : [],
      });
    }
    if (url === `/tasks/${taskId}/checklist`) {
      return Response.json({ items: [] });
    }
    if (url.startsWith(`/tasks/${taskId}/events`)) {
      return Response.json(emptyEventsPayload(taskId));
    }
    if (url === "/tasks" && method === "POST") {
      const body =
        init?.body != null && typeof init.body === "string"
          ? JSON.parse(init.body)
          : {};
      expect(body.parent_id).toBe(taskId);
      expect(body.title).toBe("Child");
      expect(body.priority).toBe("high");
      subtaskCreated = true;
      return new Response(
        JSON.stringify(
          taskDetail("child", "Child", {
            priority: "high",
            initial_prompt: "<p></p>",
          }),
        ),
        { status: 201, headers: { "Content-Type": "application/json" } },
      );
    }
    if (url === "/tasks/child/checklist/items" && method === "POST") {
      checklistPosts.push(init?.body != null ? String(init.body) : "");
      return new Response(null, { status: 204 });
    }
    return new Response("not found", { status: 404 });
  });
  return {
    checklistPosts,
  };
}

function mockTaskDetailFetchWithEvents(
  task: MockTaskDetailData,
  eventsPayload: Record<string, unknown>,
  checklistItems: unknown[] = [],
) {
  return vi
    .spyOn(globalThis, "fetch")
    .mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === `/tasks/${task.id}`) {
        return Response.json(task);
      }
      if (url === `/tasks/${task.id}/checklist`) {
        return Response.json({ items: checklistItems });
      }
      if (url.startsWith(`/tasks/${task.id}/events`)) {
        return Response.json(eventsPayload);
      }
      return new Response("not found", { status: 404 });
    });
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

  it("shows task load error with retry and refetches successfully", async () => {
    const user = userEvent.setup();
    const task = taskDetail("t1", "Recovered title");
    let taskGets = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === `/tasks/${task.id}`) {
        taskGets += 1;
        if (taskGets === 1) {
          return new Response("fail", { status: 500 });
        }
        return Response.json(task);
      }
      if (url === `/tasks/${task.id}/checklist`) {
        return Response.json({ items: [] });
      }
      if (url.startsWith(`/tasks/${task.id}/events`)) {
        return Response.json(emptyEventsPayload(task.id));
      }
      return new Response("not found", { status: 404 });
    });

    renderDetail("/tasks/t1", mockApp());

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /try again/i }));

    expect(
      await screen.findByRole("heading", { name: /^recovered title$/i }),
    ).toBeInTheDocument();
    expect(taskGets).toBe(2);
  });

  it("collapses initial prompt by default and expands on demand", async () => {
    const user = userEvent.setup();
    mockTaskDetailFetch(
      taskDetail("t1", "Testing", {
      initial_prompt: "<p>Secret long body text</p>",
      priority: "critical",
      }),
    );

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
    mockTaskDetailFetch(
      taskDetail("txss", "Unsafe prompt", {
      initial_prompt:
        '<p>Safe text</p><img src=x onerror="window.__xss = 1" /><script>window.__xss_script = 1</script><a href="javascript:alert(1)">bad</a>',
      }),
    );

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
    mockTaskDetailFetch(taskDetail("t2", "Empty prompt"));

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
    mockTaskDetailFetch(taskDetail("tb", "Blocked task", {
      status: "blocked",
    }));

    renderDetail("/tasks/tb", mockApp());

    expect(
      await screen.findByRole("heading", { name: /^blocked task$/i }),
    ).toBeInTheDocument();
    const stance = await screen.findByText("Agent needs input");
    expect(stance).toHaveAttribute("data-stance", "needs-user");
  });

  it("shows done criteria as read-only with progress counts", async () => {
    mockTaskDetailFetch(
      taskDetail("tc", "Checklist task"),
      [
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
    );

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

  it("shows checklist fetch error with try again and refetches", async () => {
    const user = userEvent.setup();
    const task = taskDetail("cf", "Checklist fetch");
    let checklistCalls = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === `/tasks/${task.id}`) {
        return Response.json(task);
      }
      if (url === `/tasks/${task.id}/checklist`) {
        checklistCalls += 1;
        if (checklistCalls === 1) {
          return new Response("server boom", { status: 500 });
        }
        return Response.json({ items: [] });
      }
      if (url.startsWith(`/tasks/${task.id}/events`)) {
        return Response.json(emptyEventsPayload(task.id));
      }
      return new Response("not found", { status: 404 });
    });

    renderDetail(`/tasks/${task.id}`, mockApp());

    expect(
      await screen.findByRole("heading", { name: /^checklist fetch$/i }),
    ).toBeInTheDocument();

    const checklistSection = document.querySelector("#task-detail-checklist");
    expect(checklistSection).not.toBeNull();
    expect(
      await within(checklistSection as HTMLElement).findByRole("alert"),
    ).toBeInTheDocument();
    await user.click(
      within(checklistSection as HTMLElement).getByRole("button", {
        name: /try again/i,
      }),
    );
    await waitFor(() => {
      expect(checklistCalls).toBe(2);
    });
    expect(
      await within(checklistSection as HTMLElement).findByText(/no criteria yet/i),
    ).toBeInTheDocument();
  });

  it("navigates to parent task after successful delete when parent_id is set", () => {
    mockTaskDetailFetch(taskDetail("sub1", "Sub", {
      parent_id: "par1",
    }));

    const app = appWithDeleteSuccess({ id: "sub1", parent_id: "par1" });

    renderDetail("/tasks/sub1", app);

    expect(mockNavigate).toHaveBeenCalledWith("/tasks/par1", { replace: true });
  });

  it("navigates home after successful delete when task has no parent", () => {
    mockTaskDetailFetch(taskDetail("root1", "Root"));

    const app = appWithDeleteSuccess({ id: "root1" });

    renderDetail("/tasks/root1", app);

    expect(mockNavigate).toHaveBeenCalledWith("/", { replace: true });
  });

  it("edits a checklist criterion via PATCH text", async () => {
    const user = userEvent.setup();
    const api = mockTaskDetailFetchWithChecklistPatch(
      taskDetail("te", "Edit checklist"),
      "item-1",
      "Before",
      "After",
    );

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

    expect(api.getPatchBody()).toBe(JSON.stringify({ text: "After" }));
    expect(await screen.findByText("After")).toBeInTheDocument();
  });

  it("lists updates newest first by seq", async () => {
    mockTaskDetailFetchWithEvents(
      taskDetail("t3", "Timeline order"),
      {
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
      },
    );

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
    const api = mockTaskDetailFetchForSubtaskCreate("parent");

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

    const childLinks = await screen.findAllByRole("link", { name: "Child" });
    const childLink =
      childLinks.find((link) =>
        link.closest(".task-subtasks-item-row"),
      ) ?? childLinks[0];
    const subtaskRow = childLink.closest(
      ".task-subtasks-item-row",
    ) as HTMLElement | null;
    expect(subtaskRow).not.toBeNull();
    expect(within(subtaskRow!).getByText("high")).toBeInTheDocument();
    expect(within(subtaskRow!).getByText("ready")).toBeInTheDocument();
    expect(api.checklistPosts).toHaveLength(1);
    expect(api.checklistPosts[0]).toContain("Criterion A");
  });

  it("links from task detail to the dedicated graph page", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/parent-graph") {
        return Response.json({
          ...taskDetail("parent-graph", "Parent graph"),
          children: [
            {
              ...taskDetail("child-a", "Child A"),
              children: [
                {
                  ...taskDetail("grandchild-a1", "Grandchild A1"),
                },
              ],
            },
            {
              ...taskDetail("child-b", "Child B"),
            },
          ],
        });
      }
      if (url === "/tasks/parent-graph/checklist") {
        return Response.json({ items: [] });
      }
      if (url.startsWith("/tasks/parent-graph/events")) {
        return Response.json(emptyEventsPayload("parent-graph"));
      }
      return new Response("not found", { status: 404 });
    });

    renderDetail("/tasks/parent-graph", mockApp());

    expect(
      await screen.findByRole("heading", { name: /^parent graph$/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /open graph view/i }),
    ).toHaveAttribute("href", "/tasks/parent-graph/graph");
  });
});
