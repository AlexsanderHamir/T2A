import { http, HttpResponse } from "msw";

/** Bootstrap absent — per-page hooks fan out to individual GETs (legacy test behaviour). */
export function bootstrapUnavailable() {
  return http.get("/v1/bootstrap", () => new HttpResponse(null, { status: 404 }));
}

export function bootstrapNetworkError() {
  return http.get("/v1/bootstrap", () => HttpResponse.error());
}

export function tasksListNetworkError() {
  return http.get("/tasks", () => HttpResponse.error());
}
