import { http, HttpResponse } from "msw";

/** Placeholder for cycle endpoints referenced by full-page flows. */
export function taskCyclesEmpty(taskId: string) {
  return http.get(`/tasks/${taskId}/cycles`, () => HttpResponse.json({ cycles: [] }));
}
