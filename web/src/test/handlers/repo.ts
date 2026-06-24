import { http, HttpResponse } from "msw";

/** Repo API unavailable — matches local dev without workspace repo. */
export function repoNotConfigured() {
  return http.get(/\/repo\//, () =>
    HttpResponse.json({ error: "repo not configured" }, { status: 503 }),
  );
}
