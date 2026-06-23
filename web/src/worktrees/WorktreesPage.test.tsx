import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi, afterEach } from "vitest";
import { DEFAULT_PROJECT_ID } from "@/types";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import { ModalStackProvider } from "@/shared/ModalStackContext";
import { requestUrl } from "@/test/requestUrl";
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

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <ModalStackProvider>
          <WorktreesPage />
        </ModalStackProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("WorktreesPage", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("shows header CTA that opens register modal when empty", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: RequestInfo | URL) => {
      const url = requestUrl(input);
      if (url.endsWith(`/projects/${DEFAULT_PROJECT_ID}/git/repositories`)) {
        return jsonResponse({ repositories: [] });
      }
      return jsonResponse({ error: "not found" }, { status: 404 });
    });

    renderPage();
    await screen.findByText(/No repositories yet/i);
    fireEvent.click(screen.getByRole("button", { name: /Register repository/i }));
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
        if (url.endsWith(`/projects/${DEFAULT_PROJECT_ID}/git/repositories`)) {
          return jsonResponse({
            repositories: [
              {
                id: repoId,
                project_id: DEFAULT_PROJECT_ID,
                path: "/repo/main",
                host_path: "",
                default_branch: "main",
                created_at: "2026-06-22T12:00:00Z",
                updated_at: "2026-06-22T12:00:00Z",
              },
            ],
          });
        }
        if (url.includes("/worktrees")) {
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
        if (url.includes("/branches")) {
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
        return jsonResponse({ error: "not found" }, { status: 404 });
    });

    renderPage();
    expect(await screen.findByText("feature")).toBeInTheDocument();
    expect(screen.getAllByText("main").length).toBeGreaterThan(0);
    expect(screen.getAllByText("/repo/main").length).toBeGreaterThan(0);
  });

  it("maps delete 409 has_running_task to dialog copy", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = requestUrl(input);
      const method = init?.method ?? "GET";
      if (method === "GET" && url.endsWith(`/projects/${DEFAULT_PROJECT_ID}/git/repositories`)) {
        return jsonResponse({
          repositories: [
            {
              id: repoId,
              project_id: DEFAULT_PROJECT_ID,
              path: "/repo/main",
              host_path: "",
              default_branch: "main",
              created_at: "2026-06-22T12:00:00Z",
              updated_at: "2026-06-22T12:00:00Z",
            },
          ],
        });
      }
      if (method === "GET" && url.includes("/worktrees") && !url.includes(wtB)) {
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
      if (method === "GET" && url.includes("/branches")) {
        return jsonResponse({ branches: [] });
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
