import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi, afterEach } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import { ModalStackProvider } from "@/shared/ModalStackContext";
import { requestUrl } from "@/test/requestUrl";
import { respondGlobalGitApi } from "@/test/handlers/gitGlobal";
import { WorktreesPage } from "./WorktreesPage";
import { RegisterRepositoryModal } from "./modals/RegisterRepositoryModal";

const repoId = "00000000-0000-4000-8000-000000000010";
const wtA = "00000000-0000-4000-8000-000000000020";
const wtB = "00000000-0000-4000-8000-000000000030";
const branchId = "00000000-0000-4000-8000-000000000040";

function jsonResponse(body: unknown, init: ResponseInit = { status: 200 }): Response {
  return new Response(JSON.stringify(body), {
    ...init,
    headers: { "content-type": "application/json", ...(init.headers ?? {}) },
  });
}

function renderPage(initialEntries: string[] = ["/worktrees"]) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={initialEntries}>
        <ModalStackProvider>
          <Routes>
            <Route path="/worktrees" element={<WorktreesPage />} />
          </Routes>
        </ModalStackProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("WorktreesPage", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("shows repository setup copy when no repositories are registered", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: RequestInfo | URL) => {
      const url = requestUrl(input);
      if (url.endsWith("/git/repositories")) {
        return jsonResponse({ repositories: [] });
      }
      const res = respondGlobalGitApi(url, "GET");
      if (res) return res;
      return jsonResponse({ error: "not found" }, { status: 404 });
    });

    renderPage();
    expect(await screen.findByRole("heading", { name: /^repositories$/i })).toBeInTheDocument();
    expect(
      await screen.findByText(/register a repository to get started/i),
    ).toBeInTheDocument();
    await userEvent.click(
      screen.getAllByRole("button", { name: /Register repository/i })[0]!,
    );
    expect(
      await screen.findByRole("button", { name: /Choose folder/i }),
    ).toBeInTheDocument();
  });

  it("shows only an error callout when repository fetch fails with Not Found", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: RequestInfo | URL) => {
      const url = requestUrl(input);
      if (url.endsWith("/git/repositories")) {
        return jsonResponse({ error: "Not Found" }, { status: 404 });
      }
      return jsonResponse({ error: "not found" }, { status: 404 });
    });

    renderPage();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(/could not load repositories/i);
    expect(alert).toHaveTextContent(/git API may be unavailable/i);
    expect(screen.queryByText(/register a repository to get started/i)).not.toBeInTheDocument();
  });

  it("opens register modal from ?register=1 and strips the query param", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: RequestInfo | URL) => {
      const url = requestUrl(input);
      if (url.endsWith("/git/repositories")) {
        return jsonResponse({ repositories: [] });
      }
      return jsonResponse({ error: "not found" }, { status: 404 });
    });

    renderPage(["/worktrees?register=1"]);
    expect(
      await screen.findByRole("button", { name: /Choose folder/i }),
    ).toBeInTheDocument();
  });

  it("renders register repository modal when open", () => {
    render(
      <ModalStackProvider>
        <RegisterRepositoryModal
          open
          pending={false}
          error={null}
          onClose={() => {}}
          onSubmit={() => {}}
        />
      </ModalStackProvider>,
    );
    expect(screen.getByRole("button", { name: /Choose folder/i })).toBeInTheDocument();
  });

  it("renders one repository with two worktrees", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: RequestInfo | URL) => {
      const url = requestUrl(input);
      if (url.endsWith("/git/repositories")) {
        return jsonResponse({
          repositories: [
            {
              id: repoId,
              path: "/repo/main",
              host_path: "",
              default_branch: "main",
              created_at: "2026-06-22T12:00:00Z",
              updated_at: "2026-06-22T12:00:00Z",
            },
          ],
        });
      }
      if (url.includes(`/git/repositories/${repoId}/worktrees`)) {
        return jsonResponse({
          worktrees: [
            {
              id: wtA,
              repository_id: repoId,
              path: "/repo/main",
              name: "main",
              is_main: true,
              created_at: "2026-06-22T12:00:00Z",
            },
            {
              id: wtB,
              repository_id: repoId,
              path: "/repo/feature",
              name: "feature",
              is_main: false,
              created_at: "2026-06-22T12:00:00Z",
            },
          ],
        });
      }
      if (url.includes(`/git/repositories/${repoId}/branches`)) {
        return jsonResponse({
          branches: [
            {
              id: branchId,
              repository_id: repoId,
              name: "main",
              head_sha: "abc123",
              created_at: "2026-06-22T12:00:00Z",
            },
          ],
        });
      }
      if (url.includes("/git/worktrees/") && url.endsWith("/branches")) {
        return jsonResponse({ associations: [] });
      }
      return jsonResponse({ error: "not found" }, { status: 404 });
    });

    renderPage();
    expect(
      await screen.findByRole("heading", {
        level: 2,
        name: /^worktrees$/i,
      }),
    ).toBeInTheDocument();
    expect(await screen.findByText("feature")).toBeInTheDocument();
    expect(screen.getAllByText("main").length).toBeGreaterThan(0);
    expect(screen.getAllByText("/repo/main").length).toBeGreaterThan(0);
  });

  it("maps delete 409 has_running_task to dialog copy", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = requestUrl(input);
      const method = init?.method ?? "GET";
      if (method === "GET" && url.endsWith("/git/repositories")) {
        return jsonResponse({
          repositories: [
            {
              id: repoId,
              path: "/repo/main",
              host_path: "",
              default_branch: "main",
              created_at: "2026-06-22T12:00:00Z",
              updated_at: "2026-06-22T12:00:00Z",
            },
          ],
        });
      }
      if (method === "GET" && url.includes(`/git/repositories/${repoId}/worktrees`)) {
        return jsonResponse({
          worktrees: [
            {
              id: wtB,
              repository_id: repoId,
              path: "/repo/feature",
              name: "feature",
              is_main: false,
              created_at: "2026-06-22T12:00:00Z",
            },
          ],
        });
      }
      if (method === "GET" && url.includes(`/git/repositories/${repoId}/branches`)) {
        return jsonResponse({ branches: [] });
      }
      if (method === "GET" && url.includes("/git/worktrees/") && url.endsWith("/branches")) {
        return jsonResponse({ associations: [] });
      }
      if (method === "DELETE") {
        return jsonResponse(
          { error: "task still running", code: "has_running_task" },
          { status: 409 },
        );
      }
      return jsonResponse({ error: "not found" }, { status: 404 });
    });

    renderPage();
    await screen.findByText("feature");
    const deleteButtons = screen.getAllByRole("button", { name: /^Delete$/i });
    await userEvent.click(deleteButtons[0]!);
    const dialog = screen.getByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: /^Delete$/i }));
    await waitFor(() => {
      expect(within(dialog).getByText(/task still running/i)).toBeInTheDocument();
    });
    expect(within(dialog).getByRole("button", { name: /^Delete$/i })).toBeDisabled();
  });
});
