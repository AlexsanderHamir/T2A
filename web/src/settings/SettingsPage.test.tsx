import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import { requestUrl } from "../test/requestUrl";
import { SettingsPage } from "./SettingsPage";

type FetchInput = RequestInfo | URL;

function jsonResponse(body: unknown, init: ResponseInit = { status: 200 }): Response {
  return new Response(JSON.stringify(body), {
    ...init,
    headers: { "content-type": "application/json", ...(init.headers ?? {}) },
  });
}

/** POST /settings/list-cursor-models is requested when Settings mounts; stub it for tests. */
function stubListCursorModelsFetch(
  inner: (input: FetchInput, init?: RequestInit) => Promise<Response>,
) {
  return async (input: FetchInput, init?: RequestInit) => {
    const u = requestUrl(input);
    if (u.endsWith("/settings/list-cursor-models")) {
      return jsonResponse({
        ok: true,
        runner: TASK_TEST_DEFAULTS.runner,
        binary_path: "/usr/local/bin/cursor-agent",
        models: [{ id: "auto", label: "Auto" }],
      });
    }
    return inner(input, init);
  };
}

function defaultSettings(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    worker_enabled: true,
    repo_root: "/Users/me/code/example",
    cursor_bin: "/usr/local/bin/cursor-agent",
    ...TASK_TEST_DEFAULTS,
    max_run_duration_seconds: 0,
    agent_pickup_delay_seconds: 5,
    updated_at: "2026-04-18T12:00:00Z",
    ...overrides,
  };
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <SettingsPage />
    </QueryClientProvider>,
  );
}

describe("SettingsPage", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("loads the settings row and pre-populates the form", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(stubListCursorModelsFetch(async (input: FetchInput) => {
      if (requestUrl(input).endsWith("/settings")) {
        return jsonResponse(defaultSettings());
      }
      return new Response("not found", { status: 404 });
    }));

    renderPage();
    const repoInput = await screen.findByLabelText(/Repository root/);
    expect(repoInput).toHaveValue("/Users/me/code/example");
    expect(screen.getByLabelText(/Enable agent worker/)).toBeChecked();
    expect(screen.getByLabelText(/Cursor CLI path/)).toHaveValue(
      "/usr/local/bin/cursor-agent",
    );
    expect(screen.getByLabelText(/Max run duration/)).toHaveValue(0);
  });

  it("formats Last saved in the selected display timezone (explicit IANA)", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(stubListCursorModelsFetch(async (input: FetchInput) => {
      if (requestUrl(input).endsWith("/settings")) {
        return jsonResponse(
          defaultSettings({
            display_timezone: "Europe/Berlin",
            // 10:00 UTC → 12:00 in Berlin on 2026-07-18 (CEST, UTC+2).
            updated_at: "2026-07-18T10:00:00Z",
          }),
        );
      }
      return new Response("not found", { status: 404 });
    }));

    renderPage();
    const chip = await screen.findByTestId("settings-last-updated");
    expect(chip.textContent).toMatch(/12:00/);
    // longOffset-style suffix for CEST (UTC+2), not a US abbreviation.
    expect(chip.textContent).toMatch(/GMT\+2|GMT\+02:00/i);
  });

  it("never PATCHes agent_paused from the form and does not render a badge for it", async () => {
    // `agent_paused` is owned by automation (agents/scripts hitting
    // PATCH /settings directly). The SettingsPage must:
    //   1. Not surface a "status" row for it — the top-bar
    //      SystemStatusChip is the single source of live agent
    //      status, and duplicating it on a configuration form
    //      confused operators (read-only row mixed into an editable
    //      form) and didn't generalize to multi-agent anyway.
    //   2. Never include agent_paused in the diff sent on Save, even
    //      after the GET response changes — otherwise saving an
    //      unrelated field would race-clobber a concurrent script
    //      that just paused the agent.
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(
        stubListCursorModelsFetch(async (input: FetchInput, init?: RequestInit) => {
          const url = requestUrl(input);
          if (url.endsWith("/settings") && (init?.method ?? "GET") === "GET") {
            return jsonResponse(defaultSettings({ agent_paused: true }));
          }
          if (url.endsWith("/settings") && init?.method === "PATCH") {
            const body = JSON.parse(String(init.body ?? "{}")) as Record<
              string,
              unknown
            >;
            // The whole point of the lockdown: agent_paused must
            // never appear in any patch this page emits, regardless
            // of what the GET returned.
            expect(body).not.toHaveProperty("agent_paused");
            return jsonResponse(
              defaultSettings({
                agent_paused: true,
                cursor_bin: "/usr/local/bin/cursor-agent-2",
                updated_at: "2026-04-18T12:34:00Z",
              }),
            );
          }
          return new Response("not found", { status: 404 });
        }),
      );

    renderPage();

    // Wait for the form to hydrate, then assert the (retired) status
    // row is gone and no stray "Paused"/"Running" pill bled through.
    await screen.findByLabelText(/Cursor CLI path/);
    expect(
      screen.queryByTestId("settings-agent-paused-status"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("settings-agent-paused"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/Agent pause status/i)).not.toBeInTheDocument();

    // Editing an unrelated field and saving must still succeed
    // without the patch including agent_paused.
    const cursorBin = screen.getByLabelText(/Cursor CLI path/);
    await userEvent.clear(cursorBin);
    await userEvent.type(cursorBin, "/usr/local/bin/cursor-agent-2");

    const saveButton = screen.getByRole("button", { name: /Save changes/i });
    await userEvent.click(saveButton);

    await waitFor(() => {
      const patches = fetchMock.mock.calls.filter(([input, init]) => {
        const u = requestUrl(input as FetchInput);
        return (
          u.endsWith("/settings") &&
          (init as RequestInit | undefined)?.method === "PATCH"
        );
      });
      expect(patches.length).toBe(1);
    });
  });

  it("shows the workspace warning banner when repo root is empty", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(stubListCursorModelsFetch(async () =>
      jsonResponse(defaultSettings({ repo_root: "" })),
    ));

    renderPage();
    expect(
      await screen.findByText(/Workspace not configured/i),
    ).toBeInTheDocument();
  });

  it("PATCHes only the changed fields and updates form state on success", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(stubListCursorModelsFetch(async (input: FetchInput, init?: RequestInit) => {
        const url = requestUrl(input);
        if (url.endsWith("/settings") && (init?.method ?? "GET") === "GET") {
          return jsonResponse(defaultSettings());
        }
        if (url.endsWith("/settings") && init?.method === "PATCH") {
          const body = JSON.parse(String(init.body ?? "{}")) as Record<string, unknown>;
          expect(Object.keys(body)).toEqual(["repo_root"]);
          expect(body.repo_root).toBe("/var/repos/new");
          return jsonResponse(
            defaultSettings({ repo_root: "/var/repos/new", updated_at: "2026-04-18T12:30:00Z" }),
          );
        }
        return new Response("not found", { status: 404 });
      }));

    renderPage();
    const repoInput = await screen.findByLabelText(/Repository root/);
    await userEvent.clear(repoInput);
    await userEvent.type(repoInput, "/var/repos/new");

    const saveBtn = screen.getByRole("button", { name: /Save changes/ });
    expect(saveBtn).not.toBeDisabled();
    await userEvent.click(saveBtn);

    await waitFor(() => expect(screen.getByTestId("settings-status")).toHaveTextContent(
      /saved/i,
    ));
    expect(fetchMock).toHaveBeenCalled();
  });

  it(
    "auto-dismisses the success banner after a few seconds",
    async () => {
      vi.spyOn(globalThis, "fetch").mockImplementation(
        stubListCursorModelsFetch(async (input: FetchInput, init?: RequestInit) => {
          const url = requestUrl(input);
          if (url.endsWith("/settings") && (init?.method ?? "GET") === "GET") {
            return jsonResponse(defaultSettings());
          }
          if (url.endsWith("/settings") && init?.method === "PATCH") {
            return jsonResponse(
              defaultSettings({
                repo_root: "/var/repos/new",
                updated_at: "2026-04-18T12:30:00Z",
              }),
            );
          }
          return new Response("not found", { status: 404 });
        }),
      );

      renderPage();
      const repoInput = await screen.findByLabelText(/Repository root/);
      await userEvent.clear(repoInput);
      await userEvent.type(repoInput, "/var/repos/new");
      await userEvent.click(screen.getByRole("button", { name: /Save changes/ }));

      await waitFor(() =>
        expect(screen.getByTestId("settings-status")).toHaveTextContent(/saved/i),
      );

      await waitFor(
        () => expect(screen.queryByTestId("settings-status")).not.toBeInTheDocument(),
        { timeout: 6_000 },
      );
    },
    12_000,
  );

  it("disables Save when no fields have changed", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(stubListCursorModelsFetch(async () =>
      jsonResponse(defaultSettings()),
    ));
    renderPage();
    const saveBtn = await screen.findByRole("button", { name: /Save changes/ });
    expect(saveBtn).toBeDisabled();
  });

  it("calls /settings/probe-cursor and shows the version on success", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(stubListCursorModelsFetch(async (input: FetchInput, init?: RequestInit) => {
      const url = requestUrl(input);
      if (url.endsWith("/settings/probe-cursor")) {
        return jsonResponse({ ok: true, runner: TASK_TEST_DEFAULTS.runner, version: "2026.04" });
      }
      if (url.endsWith("/settings") && (init?.method ?? "GET") === "GET") {
        return jsonResponse(defaultSettings());
      }
      return new Response("not found", { status: 404 });
    }));

    renderPage();
    const probeBtn = await screen.findByRole("button", { name: /Test cursor binary/ });
    await userEvent.click(probeBtn);
    await waitFor(() =>
      expect(screen.getByTestId("settings-status")).toHaveTextContent(/2026\.04/),
    );
  });

  it("surfaces the PATH-resolved binary path when the field is blank, in both the status and the help text", async () => {
    // Without this, an operator who leaves the cursor-bin field blank
    // and clicks Test sees only "Cursor binary OK" and has no idea
    // which binary on PATH was actually exec'd. The /settings/probe-cursor
    // response now carries `binary_path`; the SPA must surface it.
    vi.spyOn(globalThis, "fetch").mockImplementation(
      stubListCursorModelsFetch(async (input: FetchInput, init?: RequestInit) => {
        const url = requestUrl(input);
        if (url.endsWith("/settings/probe-cursor")) {
          return jsonResponse({
            ok: true,
            runner: TASK_TEST_DEFAULTS.runner,
            binary_path: "/opt/local/bin/cursor-agent",
            version: "2026.05",
          });
        }
        if (url.endsWith("/settings") && (init?.method ?? "GET") === "GET") {
          return jsonResponse(defaultSettings({ cursor_bin: "" }));
        }
        return new Response("not found", { status: 404 });
      }),
    );

    renderPage();
    const probeBtn = await screen.findByRole("button", {
      name: /Test cursor binary/,
    });
    await userEvent.click(probeBtn);
    await waitFor(() =>
      expect(screen.getByTestId("settings-status")).toHaveTextContent(
        /at \/opt\/local\/bin\/cursor-agent.*2026\.05/,
      ),
    );
    expect(
      screen.getByTestId("settings-resolved-cursor-bin"),
    ).toHaveTextContent("/opt/local/bin/cursor-agent");
  });

  it("surfaces probe failures via the error channel (role='alert', not role='status')", async () => {
    // Session #36 — probe `{ ok: false, error }` is semantically a
    // failure and must announce assertively to screen-readers, AND
    // must NOT appear in the success-styled `settings-status`
    // region (which is now reserved for actual successes).
    vi.spyOn(globalThis, "fetch").mockImplementation(stubListCursorModelsFetch(async (input: FetchInput, init?: RequestInit) => {
      const url = requestUrl(input);
      if (url.endsWith("/settings/probe-cursor")) {
        return jsonResponse({ ok: false, runner: TASK_TEST_DEFAULTS.runner, error: "spawn ENOENT" });
      }
      if (url.endsWith("/settings") && (init?.method ?? "GET") === "GET") {
        return jsonResponse(defaultSettings());
      }
      return new Response("not found", { status: 404 });
    }));

    renderPage();
    const probeBtn = await screen.findByRole("button", { name: /Test cursor binary/ });
    await userEvent.click(probeBtn);
    await waitFor(() =>
      expect(screen.getByTestId("settings-status-error")).toHaveTextContent(
        /spawn ENOENT/,
      ),
    );
    expect(screen.queryByTestId("settings-status")).not.toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent(/spawn ENOENT/);
  });

  it("surfaces patch errors via the error channel and does not show the success status", async () => {
    // Session #36 regression for the failed-PATCH case: previously
    // a 500 from PATCH /settings rendered through the same
    // `role="status"` channel as a successful save, which was a
    // direct a11y regression (assertive failure announced as polite).
    vi.spyOn(globalThis, "fetch").mockImplementation(stubListCursorModelsFetch(async (input: FetchInput, init?: RequestInit) => {
      const url = requestUrl(input);
      if (url.endsWith("/settings") && (init?.method ?? "GET") === "GET") {
        return jsonResponse(defaultSettings());
      }
      if (url.endsWith("/settings") && init?.method === "PATCH") {
        return new Response(
          JSON.stringify({ error: "internal: disk full" }),
          {
            status: 500,
            headers: { "content-type": "application/json" },
          },
        );
      }
      return new Response("not found", { status: 404 });
    }));

    renderPage();
    const repoInput = await screen.findByLabelText(/Repository root/);
    await userEvent.clear(repoInput);
    await userEvent.type(repoInput, "/var/repos/new");

    const saveBtn = screen.getByRole("button", { name: /Save changes/ });
    await userEvent.click(saveBtn);

    await waitFor(() =>
      expect(screen.getByTestId("settings-status-error")).toBeInTheDocument(),
    );
    // The error message text varies by API client error shape, but
    // the assertive alert region must always render so screen
    // readers announce the failure.
    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.queryByTestId("settings-status")).not.toBeInTheDocument();
  });

  it("preserves in-flight typing on other fields when a PATCH resolves", async () => {
    // Session #37 regression: if the user edits field A, hits Save,
    // and then keeps typing in field B while the PATCH is still in
    // flight, the post-resolution `setForm(toFormState(next))` used
    // to clobber field B back to its server value (silently losing
    // the user's typing). The fix snapshots `formAtSubmit` and only
    // applies server truth per-field where the form hasn't been
    // re-edited since submit.
    let releasePatch: ((value: Response) => void) | null = null;
    vi.spyOn(globalThis, "fetch").mockImplementation(
      stubListCursorModelsFetch(async (input: FetchInput, init?: RequestInit) => {
        const url = requestUrl(input);
        if (url.endsWith("/settings") && (init?.method ?? "GET") === "GET") {
          return jsonResponse(defaultSettings());
        }
        if (url.endsWith("/settings") && init?.method === "PATCH") {
          // Hold the PATCH response until the test releases it, so
          // we have a deterministic in-flight window for typing
          // into the second field.
          return new Promise<Response>((resolve) => {
            releasePatch = resolve;
          });
        }
        return new Response("not found", { status: 404 });
      }),
    );

    renderPage();
    const repoInput = await screen.findByLabelText(/Repository root/);
    await userEvent.clear(repoInput);
    await userEvent.type(repoInput, "/var/repos/new");

    const saveBtn = screen.getByRole("button", { name: /Save changes/ });
    await userEvent.click(saveBtn);

    // PATCH is in flight; the user keeps typing in the cursor-bin
    // field (a field NOT in the submitted patch body).
    const cursorInput = screen.getByLabelText(/Cursor CLI path/);
    await userEvent.clear(cursorInput);
    await userEvent.type(cursorInput, "/opt/local/bin/cursor-agent");

    // Now release the PATCH. The response carries the server's
    // pre-edit cursor_bin (because the user's in-flight typing was
    // never sent). Without the race-hardening, the form would now
    // overwrite the cursor field back to the server value.
    if (!releasePatch) throw new Error("PATCH was not in flight");
    (releasePatch as (value: Response) => void)(
      jsonResponse(
        defaultSettings({
          repo_root: "/var/repos/new",
          updated_at: "2026-04-19T12:30:00Z",
        }),
      ),
    );

    await waitFor(() =>
      expect(screen.getByTestId("settings-status")).toHaveTextContent(/saved/i),
    );
    // The user's in-flight typing in cursor_bin must survive.
    expect(cursorInput).toHaveValue("/opt/local/bin/cursor-agent");
    // The patched field (repo_root) is whatever the user submitted
    // (no further edits), so the server value applies cleanly.
    expect(repoInput).toHaveValue("/var/repos/new");
    // Form is now dirty against the new server baseline (user has
    // unsaved cursor_bin changes), so Save re-enables for the next
    // round.
    expect(screen.getByRole("button", { name: /Save changes/ })).not.toBeDisabled();
  });

  it("preserves user re-edits to the same field while a PATCH is in flight", async () => {
    // Session #37 regression: the user types /A, hits Save, then
    // changes their mind to /B while the PATCH (carrying /A) is
    // still in flight. The PATCH resolves with /A. The user's
    // current intent is /B; clobbering back to /A would be silent
    // data loss + violate the user's mental model.
    let releasePatch: ((value: Response) => void) | null = null;
    vi.spyOn(globalThis, "fetch").mockImplementation(
      stubListCursorModelsFetch(async (input: FetchInput, init?: RequestInit) => {
        const url = requestUrl(input);
        if (url.endsWith("/settings") && (init?.method ?? "GET") === "GET") {
          return jsonResponse(defaultSettings());
        }
        if (url.endsWith("/settings") && init?.method === "PATCH") {
          return new Promise<Response>((resolve) => {
            releasePatch = resolve;
          });
        }
        return new Response("not found", { status: 404 });
      }),
    );

    renderPage();
    const repoInput = await screen.findByLabelText(/Repository root/);
    await userEvent.clear(repoInput);
    await userEvent.type(repoInput, "/var/repos/A");

    await userEvent.click(screen.getByRole("button", { name: /Save changes/ }));

    await userEvent.clear(repoInput);
    await userEvent.type(repoInput, "/var/repos/B");

    if (!releasePatch) throw new Error("PATCH was not in flight");
    (releasePatch as (value: Response) => void)(
      jsonResponse(
        defaultSettings({
          repo_root: "/var/repos/A",
          updated_at: "2026-04-19T12:30:00Z",
        }),
      ),
    );

    await waitFor(() =>
      expect(screen.getByTestId("settings-status")).toHaveTextContent(/saved/i),
    );
    expect(repoInput).toHaveValue("/var/repos/B");
  });

  it("rejects negative max_run_duration_seconds", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(stubListCursorModelsFetch(async () =>
      jsonResponse(defaultSettings()),
    ));
    renderPage();
    const maxInput = await screen.findByLabelText(/Max run duration/);
    await userEvent.clear(maxInput);
    await userEvent.type(maxInput, "-5");
    expect(screen.getByRole("alert")).toHaveTextContent(
      /non-negative integer/i,
    );
    expect(screen.getByRole("button", { name: /Save changes/ })).toBeDisabled();
  });
});
