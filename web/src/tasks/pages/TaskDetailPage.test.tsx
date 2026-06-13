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
    deleteSuccess: false,
    deleteVariables: undefined,
    openEdit: vi.fn(),
    requestDelete: vi.fn(),
    saving: false,
  } as unknown as ReturnType<typeof useTasksApp>;
}

function appWithDeleteSuccess(
  variables: { id: string },
): ReturnType<typeof useTasksApp> {
  return {
    ...mockApp(),
    deleteSuccess: true,
    deleteVariables: variables,
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
  runner?: string;
  cursor_model?: string;
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
    runner: "cursor",
    cursor_model: "",
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
    expect(
      await screen.findByText(/no agent is waiting on you for this task right now/i),
    ).toBeInTheDocument();

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

  it("surfaces the needs-user signal via the attention callout and pill", async () => {
    // Redesign (2026-06-04): the header no longer carries a standalone
    // stance line. A needs-user task is signalled by (a) the rich
    // attention callout and (b) the highlighted status pill.
    mockTaskDetailFetch(taskDetail("tb", "Blocked task", {
      status: "blocked",
    }));

    renderDetail("/tasks/tb", mockApp());

    expect(
      await screen.findByRole("heading", { name: /^blocked task$/i }),
    ).toBeInTheDocument();
    expect(await screen.findByText(/the agent is blocked/i)).toBeInTheDocument();
    expect(
      screen.getByText("Blocked", { selector: ".ui-badge" }),
    ).toHaveAttribute(
      "data-needs-user",
      "true",
    );
    expect(screen.queryByText("Agent needs input")).not.toBeInTheDocument();
  });

  // OK-line dot tone is derived from task.status on the page so two
  // "all clear" tasks (e.g. done vs running) render the same copy but
  // a distinct dot colour. Pin a couple of representative statuses.
  it("colours the OK-line dot per task.status (done -> success, running -> active, on_hold -> caution)", async () => {
    for (const { id, status, expectedTone } of [
      { id: "td-done", status: "done", expectedTone: "success" },
      { id: "td-run", status: "running", expectedTone: "active" },
      { id: "td-hold", status: "on_hold", expectedTone: "caution" },
    ] as const) {
      mockTaskDetailFetch(taskDetail(id, `Task ${id}`, { status }));
      const { unmount } = renderDetail(
        `/tasks/${id}`,
        mockApp(),
      );

      const ok = await screen.findByText(/no agent is waiting on you/i);
      expect(ok).toHaveAttribute("data-tone", expectedTone);
      unmount();
    }
  });

  // The Dependencies section is always present so the absence of upstream
  // tasks is stated explicitly rather than rendering nothing. (2026-06-04:
  // reverted an earlier "hide when empty" pass per product feedback.)
  it("always renders the Dependencies section, with an empty state when there are none", async () => {
    mockTaskDetailFetch(taskDetail("tnd", "No deps task"));

    renderDetail("/tasks/tnd", mockApp());

    expect(
      await screen.findByRole("heading", { name: /^no deps task$/i }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("task-deps-empty")).toBeInTheDocument();
    expect(
      screen.getByText(/no upstream dependencies/i),
    ).toBeInTheDocument();
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

  it("navigates home after successful delete", () => {
    mockTaskDetailFetch(taskDetail("root1", "Root"));

    const app = appWithDeleteSuccess({ id: "root1" });

    renderDetail("/tasks/root1", app);

    expect(mockNavigate).toHaveBeenCalledWith("/", { replace: true });
  });

  it("disables Add criterion when the task is running or done", async () => {
    mockTaskDetailFetch(taskDetail("tr", "Running task", { status: "running" }));

    renderDetail("/tasks/tr", mockApp());

    expect(
      await screen.findByRole("heading", { name: /^running task$/i }),
    ).toBeInTheDocument();

    const addBtn = screen.getByRole("button", { name: /^add criterion$/i });
    expect(addBtn).toBeDisabled();
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
});
