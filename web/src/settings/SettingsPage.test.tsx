import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { requestUrl } from "../test/requestUrl";
import { SettingsPage } from "./SettingsPage";

type FetchInput = RequestInfo | URL;

function jsonResponse(body: unknown, init: ResponseInit = { status: 200 }): Response {
  return new Response(JSON.stringify(body), {
    ...init,
    headers: { "content-type": "application/json", ...(init.headers ?? {}) },
  });
}

function defaultSettings(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    worker_enabled: true,
    runner: "cursor",
    repo_root: "/Users/me/code/example",
    cursor_bin: "/usr/local/bin/cursor-agent",
    max_run_duration_seconds: 0,
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
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: FetchInput) => {
      if (requestUrl(input).endsWith("/settings")) {
        return jsonResponse(defaultSettings());
      }
      return new Response("not found", { status: 404 });
    });

    renderPage();
    const repoInput = await screen.findByLabelText(/Repository root/);
    expect(repoInput).toHaveValue("/Users/me/code/example");
    expect(screen.getByLabelText(/Enable agent worker/)).toBeChecked();
    expect(screen.getByLabelText(/Cursor CLI path/)).toHaveValue(
      "/usr/local/bin/cursor-agent",
    );
    expect(screen.getByLabelText(/Max run duration/)).toHaveValue(0);
  });

  it("shows the workspace warning banner when repo root is empty", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async () =>
      jsonResponse(defaultSettings({ repo_root: "" })),
    );

    renderPage();
    expect(
      await screen.findByText(/Workspace not configured/i),
    ).toBeInTheDocument();
  });

  it("PATCHes only the changed fields and updates form state on success", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input: FetchInput, init?: RequestInit) => {
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
      });

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

  it("disables Save when no fields have changed", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async () =>
      jsonResponse(defaultSettings()),
    );
    renderPage();
    const saveBtn = await screen.findByRole("button", { name: /Save changes/ });
    expect(saveBtn).toBeDisabled();
  });

  it("calls /settings/probe-cursor and shows the version on success", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: FetchInput, init?: RequestInit) => {
      const url = requestUrl(input);
      if (url.endsWith("/settings/probe-cursor")) {
        return jsonResponse({ ok: true, runner: "cursor", version: "2026.04" });
      }
      if (url.endsWith("/settings") && (init?.method ?? "GET") === "GET") {
        return jsonResponse(defaultSettings());
      }
      return new Response("not found", { status: 404 });
    });

    renderPage();
    const probeBtn = await screen.findByRole("button", { name: /Test cursor binary/ });
    await userEvent.click(probeBtn);
    await waitFor(() =>
      expect(screen.getByTestId("settings-status")).toHaveTextContent(/2026\.04/),
    );
  });

  it("surfaces probe failures without throwing", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: FetchInput, init?: RequestInit) => {
      const url = requestUrl(input);
      if (url.endsWith("/settings/probe-cursor")) {
        return jsonResponse({ ok: false, runner: "cursor", error: "spawn ENOENT" });
      }
      if (url.endsWith("/settings") && (init?.method ?? "GET") === "GET") {
        return jsonResponse(defaultSettings());
      }
      return new Response("not found", { status: 404 });
    });

    renderPage();
    const probeBtn = await screen.findByRole("button", { name: /Test cursor binary/ });
    await userEvent.click(probeBtn);
    await waitFor(() =>
      expect(screen.getByTestId("settings-status")).toHaveTextContent(/spawn ENOENT/),
    );
  });

  it("calls /settings/cancel-current-run and reports whether a run was cancelled", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: FetchInput, init?: RequestInit) => {
      const url = requestUrl(input);
      if (url.endsWith("/settings/cancel-current-run")) {
        return jsonResponse({ cancelled: true });
      }
      if (url.endsWith("/settings") && (init?.method ?? "GET") === "GET") {
        return jsonResponse(defaultSettings());
      }
      return new Response("not found", { status: 404 });
    });

    renderPage();
    const cancelBtn = await screen.findByRole("button", { name: /Cancel current run/ });
    await userEvent.click(cancelBtn);
    await waitFor(() =>
      expect(screen.getByTestId("settings-status")).toHaveTextContent(
        /cancelled/i,
      ),
    );
  });

  it("rejects negative max_run_duration_seconds", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async () =>
      jsonResponse(defaultSettings()),
    );
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
