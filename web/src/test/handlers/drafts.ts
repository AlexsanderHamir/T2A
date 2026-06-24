import { http, HttpResponse } from "msw";
import { createDeferred } from "@/test/deferred";

export type DraftSummary = {
  id: string;
  name: string;
  created_at: string;
  updated_at: string;
};

export function draftsListEmpty() {
  return http.get("/task-drafts", () => HttpResponse.json({ drafts: [] }));
}

export function draftsList(drafts: DraftSummary[]) {
  return http.get("/task-drafts", () => HttpResponse.json({ drafts }));
}

/** @returns [mswHandler, deferred controls for resolve/reject in test] */
export function draftsListPending() {
  const deferred = createDeferred<Response>();
  return [
    http.get("/task-drafts", () => deferred.promise),
    deferred,
  ] as const;
}

export function draftGet(
  id: string,
  payload: {
    name: string;
    created_at: string;
    updated_at: string;
    payload?: Record<string, unknown>;
  },
) {
  return http.get(`/task-drafts/${id}`, () =>
    HttpResponse.json({ id, ...payload }),
  );
}

export function draftCreate(
  status: number,
  body: Record<string, unknown> | { error: string },
) {
  return http.post("/task-drafts", () =>
    HttpResponse.json(body, {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

export function draftCreateOk(summary: DraftSummary) {
  return http.post("/task-drafts", () =>
    HttpResponse.json(summary, {
      status: summary.id ? 200 : 201,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

export function draftCreateCapture(
  onPost: (body: string) => void,
  response: { status: number; body: Record<string, unknown> },
) {
  return http.post("/task-drafts", async ({ request }) => {
    onPost(await request.text());
    return HttpResponse.json(response.body, {
      status: response.status,
      headers: { "Content-Type": "application/json" },
    });
  });
}

export function draftDelete(id: string, status: number, body?: { error: string }) {
  return http.delete(`/task-drafts/${id}`, () =>
    body
      ? HttpResponse.json(body, { status, headers: { "Content-Type": "application/json" } })
      : new HttpResponse(null, { status }),
  );
}

export function draftDeletePending(id: string) {
  const deferred = createDeferred<Response>();
  return [
    http.delete(`/task-drafts/${id}`, () => deferred.promise),
    deferred,
  ] as const;
}
