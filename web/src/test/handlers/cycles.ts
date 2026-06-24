import { http, HttpResponse } from "msw";

/** Placeholder for cycle endpoints referenced by full-page flows. */
export function taskCyclesEmpty(taskId: string) {
  return http.get(`/tasks/${taskId}/cycles`, () => HttpResponse.json({ cycles: [] }));
}

export function cycleDetailGet(
  taskId: string,
  cycleId: string,
  body: unknown,
) {
  return http.get(`/tasks/${taskId}/cycles/${cycleId}`, () =>
    HttpResponse.json(body),
  );
}

export function cycleStreamGet(
  taskId: string,
  cycleId: string,
  events: unknown[],
) {
  return http.get(`/tasks/${taskId}/cycles/${cycleId}/stream`, () =>
    HttpResponse.json({
      task_id: taskId,
      cycle_id: cycleId,
      events,
      limit: 500,
      has_more: false,
    }),
  );
}

export function cyclePageAuditEvents(taskId: string, events: unknown[]) {
  return http.get(`/tasks/${taskId}/events`, ({ request }) => {
    const url = new URL(request.url);
    if (url.searchParams.get("limit") !== "200") {
      return new HttpResponse(null, { status: 404 });
    }
    return HttpResponse.json({
      task_id: taskId,
      events,
      approval_pending: false,
    });
  });
}

/** Handlers for TaskCycleDetailPage routing tests. */
export function cyclePageHandlers(options: {
  taskId: string;
  cycleId: string;
  cycle: unknown;
  streamEvents: unknown[];
  auditEvents: unknown[];
}) {
  const { taskId, cycleId, cycle, streamEvents, auditEvents } = options;
  return [
    cycleDetailGet(taskId, cycleId, cycle),
    cycleStreamGet(taskId, cycleId, streamEvents),
    cyclePageAuditEvents(taskId, auditEvents),
  ];
}
