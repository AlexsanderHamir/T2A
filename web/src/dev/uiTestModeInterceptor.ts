import { isUiTestMode } from "./uiTestMode";
import {
  demoContextWire,
  demoCycleFailuresWire,
  demoGoalsWire,
  demoProjectWire,
  demoProjectsListWire,
  demoStepsWire,
  demoTaskChecklistWire,
  demoTaskCyclesListWire,
  demoTaskDraftsWire,
  demoTaskEventsWire,
  demoTaskStatsWire,
  demoTasksListWire,
  demoTaskWire,
  isDemoProjectId,
  isDemoTaskId,
} from "./uiTestModeDemoWire";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function parseRequestUrl(input: RequestInfo | URL, init?: RequestInit): URL | null {
  const method = (init?.method ?? "GET").toUpperCase();
  if (method !== "GET") return null;
  if (typeof input === "string") {
    if (input.startsWith("http://") || input.startsWith("https://")) {
      return new URL(input);
    }
    return new URL(input, "http://ui-test.local");
  }
  if (input instanceof URL) return input;
  if (input instanceof Request) return new URL(input.url);
  return null;
}

/**
 * When UI test mode is active, returns a synthetic JSON Response for matching
 * GET requests so the SPA can render without taskapi data. Returns null to
 * use the real network (mutations, health, settings, unknown paths).
 */
export function interceptUiTestModeFetch(
  input: RequestInfo | URL,
  init?: RequestInit,
): Response | null {
  if (!isUiTestMode()) return null;
  const u = parseRequestUrl(input, init);
  if (!u) return null;
  const path = u.pathname;
  const search = u.searchParams;

  if (path === "/projects") {
    return jsonResponse(demoProjectsListWire());
  }

  const projectOnly = path.match(/^\/projects\/([^/]+)$/);
  if (projectOnly) {
    const id = decodeURIComponent(projectOnly[1] ?? "");
    const row = demoProjectWire(id);
    if (row) return jsonResponse(row);
    return null;
  }

  const ctx = path.match(/^\/projects\/([^/]+)\/context$/);
  if (ctx) {
    const id = decodeURIComponent(ctx[1] ?? "");
    if (!isDemoProjectId(id)) return null;
    return jsonResponse(demoContextWire(id));
  }

  const goals = path.match(/^\/projects\/([^/]+)\/goals$/);
  if (goals) {
    const id = decodeURIComponent(goals[1] ?? "");
    if (!isDemoProjectId(id)) return null;
    return jsonResponse(demoGoalsWire(id));
  }

  const steps = path.match(/^\/projects\/([^/]+)\/steps$/);
  if (steps) {
    const id = decodeURIComponent(steps[1] ?? "");
    if (!isDemoProjectId(id)) return null;
    return jsonResponse(demoStepsWire(id));
  }

  if (path === "/tasks/stats") {
    return jsonResponse(demoTaskStatsWire());
  }

  if (path === "/tasks") {
    const limit = Number(search.get("limit") ?? "200") || 200;
    const offset = Number(search.get("offset") ?? "0") || 0;
    const afterId = search.get("after_id");
    return jsonResponse(demoTasksListWire(limit, offset, afterId));
  }

  if (path.startsWith("/tasks/cycle-failures")) {
    return jsonResponse(demoCycleFailuresWire());
  }

  if (path.startsWith("/task-drafts")) {
    return jsonResponse(demoTaskDraftsWire());
  }

  const checklist = path.match(/^\/tasks\/([^/]+)\/checklist$/);
  if (checklist) {
    const tid = decodeURIComponent(checklist[1] ?? "");
    if (!isDemoTaskId(tid)) return null;
    return jsonResponse(demoTaskChecklistWire());
  }

  const events = path.match(/^\/tasks\/([^/]+)\/events$/);
  if (events) {
    const tid = decodeURIComponent(events[1] ?? "");
    if (!isDemoTaskId(tid)) return null;
    return jsonResponse(demoTaskEventsWire(tid));
  }

  const cyclesList = path.match(/^\/tasks\/([^/]+)\/cycles$/);
  if (cyclesList) {
    const tid = decodeURIComponent(cyclesList[1] ?? "");
    if (!isDemoTaskId(tid)) return null;
    return jsonResponse(demoTaskCyclesListWire(tid));
  }

  const taskOne = path.match(/^\/tasks\/([^/]+)$/);
  if (taskOne) {
    const tid = decodeURIComponent(taskOne[1] ?? "");
    const row = demoTaskWire(tid);
    if (row) return jsonResponse(row);
    return null;
  }

  return null;
}
