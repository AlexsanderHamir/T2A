import { http, HttpResponse } from "msw";
import type { Task } from "@/types/task";
import { createDeferred } from "@/test/deferred";
import { makeTask } from "@/test/taskDefaults";
import { gitApiHandlers } from "@/test/handlers/gitMsw";
import { globalGitApiHandlers } from "@/test/handlers/gitMsw";
import { TASK_LIST_PAGE_SIZE } from "@/tasks/task-paging";

const emptyTaskStats = {
  total: 0,
  ready: 0,
  critical: 0,
  scheduled: 0,
  by_status: {},
  by_priority: {},
  cycles: { by_status: {}, by_triggered_by: {} },
  phases: { by_phase_status: { execute: {}, verify: {} } },
  runner: {
    by_runner: {},
    by_model: {},
    by_runner_model: {},
    by_runner_model_resolved: {},
  },
  recent_failures: [],
};

function taskListJson(tasks: Task[]) {
  return HttpResponse.json({
    tasks,
    limit: TASK_LIST_PAGE_SIZE,
    offset: 0,
    has_more: false,
  });
}

export function taskStatsEmpty() {
  return http.get("/tasks/stats", () => HttpResponse.json(emptyTaskStats));
}

export function tasksListEmpty() {
  return http.get("/tasks", () => taskListJson([]));
}

export function tasksList(tasks: Task[]) {
  return http.get("/tasks", () => taskListJson(tasks));
}

export function taskGet(id: string, task: Partial<Task> & Pick<Task, "id" | "title">) {
  return http.get(`/tasks/${id}`, () =>
    HttpResponse.json({
      initial_prompt: "",
      status: "ready",
      priority: "medium",
      checklist_inherit: false,
      ...task,
    }),
  );
}

/** Keeps GET /tasks/:id pending until deferred.resolve/reject is called. */
export function taskGetPending(taskId: string) {
  const deferred = createDeferred<Response>();
  const handler = http.get(`/tasks/${taskId}`, () => deferred.promise);
  return [handler, deferred] as const;
}

export function taskGetFlaky(
  taskId: string,
  task: Partial<Task> & Pick<Task, "id" | "title">,
) {
  let calls = 0;
  return http.get(`/tasks/${taskId}`, () => {
    calls += 1;
    if (calls === 1) {
      return new HttpResponse(null, { status: 500 });
    }
    return HttpResponse.json({
      initial_prompt: "",
      status: "ready",
      priority: "medium",
      checklist_inherit: false,
      ...task,
    });
  });
}

export function taskChecklist(taskId: string, items: unknown[]) {
  return http.get(`/tasks/${taskId}/checklist`, () =>
    HttpResponse.json({ items }),
  );
}

export function taskChecklistFlaky(taskId: string) {
  let calls = 0;
  return http.get(`/tasks/${taskId}/checklist`, () => {
    calls += 1;
    if (calls === 1) {
      return new HttpResponse(null, { status: 500 });
    }
    return HttpResponse.json({ items: [] });
  });
}

export function taskChecklistItemPatch(
  taskId: string,
  itemId: string,
  onPatch: (body: string) => void,
  nextText: string,
) {
  return http.patch(
    `/tasks/${taskId}/checklist/items/${itemId}`,
    async ({ request }) => {
      const body = await request.text();
      onPatch(body);
      return HttpResponse.json({
        items: [
          {
            id: itemId,
            sort_order: 0,
            text: nextText,
            done: false,
          },
        ],
      });
    },
  );
}

export function taskEventGet(taskId: string, seq: number, body: unknown) {
  return http.get(`/tasks/${taskId}/events/${seq}`, () =>
    HttpResponse.json(body),
  );
}

export function taskEventGetFlaky(taskId: string, seq: number, body: unknown) {
  let calls = 0;
  return http.get(`/tasks/${taskId}/events/${seq}`, () => {
    calls += 1;
    if (calls === 1) {
      return new HttpResponse(null, { status: 500 });
    }
    return HttpResponse.json(body);
  });
}

export function taskChecklistEmpty(taskId: string) {
  return http.get(`/tasks/${taskId}/checklist`, () =>
    HttpResponse.json({ items: [] }),
  );
}

export function taskEventsEmpty(taskId: string) {
  return http.get(new RegExp(`/tasks/${taskId}/events`), () =>
    HttpResponse.json({
      task_id: taskId,
      events: [],
      limit: 20,
      total: 0,
      has_more_newer: false,
      has_more_older: false,
      approval_pending: false,
    }),
  );
}

/** Task detail page event timeline — exact list path only (not /events/:seq). */
export function taskEventsListEmpty(taskId: string) {
  return http.get(`/tasks/${taskId}/events`, () =>
    HttpResponse.json({
      task_id: taskId,
      events: [],
      limit: 20,
      total: 0,
      has_more_newer: false,
      has_more_older: false,
      approval_pending: false,
    }),
  );
}

export function taskCreate(handler: (body: unknown) => Task) {
  return http.post("/tasks", async ({ request }) => {
    const body = await request.json();
    const task = handler(body);
    return HttpResponse.json(task, { status: 201 });
  });
}

export function taskCreateFixed(task: Task) {
  return taskCreate(() => task);
}

export function checklistItemCreate(taskId: string) {
  return http.post(`/tasks/${taskId}/checklist/items`, () =>
    new HttpResponse(null, { status: 204 }),
  );
}

export function defaultTask(id = "t1", title = "Ship fix"): Task {
  return makeTask({ id, title, initial_prompt: "" });
}

/** Handlers for home create-modal flows that refresh the task list after POST. */
export function taskCreateFlowHandlers(options: {
  taskId: string;
  title: string;
  /** Tasks already on the home list before create (e.g. parent picker scenarios). */
  seedTasks?: Task[];
  onPost?: (body: unknown) => void;
}) {
  let created = false;
  const task = defaultTask(options.taskId, options.title);
  const seed = options.seedTasks ?? [];
  return [
    ...gitApiHandlers(),
    ...globalGitApiHandlers(),
    http.get("/tasks", () => {
      const tasks = created ? [...seed, task] : seed;
      return taskListJson(tasks);
    }),
    http.post("/tasks", async ({ request }) => {
      created = true;
      const body = await request.json();
      options.onPost?.(body);
      return HttpResponse.json(task, { status: 201 });
    }),
    checklistItemCreate(options.taskId),
  ];
}
