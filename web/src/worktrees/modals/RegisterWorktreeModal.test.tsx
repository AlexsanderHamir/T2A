import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { gitQueryKeys } from "@/lib/gitQueryKeys";
import { RegisterWorktreeModal } from "./RegisterWorktreeModal";

function jsonResponse(body: unknown, init: ResponseInit = { status: 200 }): Response {
  return new Response(JSON.stringify(body), {
    ...init,
    headers: { "content-type": "application/json", ...(init.headers ?? {}) },
  });
}

describe("RegisterWorktreeModal", () => {
  it("auto-reconciles with loading status when live worktree inventory cannot load", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.includes("/worktrees/live")) {
        return jsonResponse({ error: "open repository: path missing" }, { status: 500 });
      }
      if (url.includes("/branches")) {
        return jsonResponse({ branches: [] });
      }
      return new Response("not found", { status: 404 });
    });

    const onReconcile = vi.fn();
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 } },
    });

    render(
      <QueryClientProvider client={client}>
        <RegisterWorktreeModal
          open
          pending={false}
          error={null}
          repositoryId="00000000-0000-4000-8000-000000000010"
          storedPath="/stale/old-checkout"
          onReconcile={onReconcile}
          onClose={() => {}}
          onSubmit={() => {}}
        />
      </QueryClientProvider>,
    );

    expect(
      await screen.findByText(/registered checkout path isn't available on disk/i),
    ).toBeInTheDocument();
    expect(await screen.findByRole("status")).toHaveTextContent(
      /Syncing registered worktrees with git/i,
    );
    await waitFor(() => expect(onReconcile).toHaveBeenCalledTimes(1));
    expect(
      screen.queryByRole("button", { name: /Reconcile repository/i }),
    ).not.toBeInTheDocument();

    fetchMock.mockRestore();
  });

  it("does not auto-reconcile when bootstrap relocate is pending", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.includes("/worktrees/live")) {
        return jsonResponse({ error: "open repository: path missing" }, { status: 500 });
      }
      if (url.includes("/branches")) {
        return jsonResponse({ branches: [] });
      }
      return new Response("not found", { status: 404 });
    });

    const onReconcile = vi.fn();
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 } },
    });

    render(
      <QueryClientProvider client={client}>
        <RegisterWorktreeModal
          open
          pending={false}
          error={null}
          repositoryId="00000000-0000-4000-8000-000000000010"
          storedPath="/stale/old-checkout"
          reconcileBlocked
          onReconcile={onReconcile}
          onClose={() => {}}
          onSubmit={() => {}}
        />
      </QueryClientProvider>,
    );

    expect(
      await screen.findByText(/registered checkout path isn't available on disk/i),
    ).toBeInTheDocument();
    expect(onReconcile).not.toHaveBeenCalled();

    fetchMock.mockRestore();
  });

  it("does not auto-reconcile again when live inventory refetches", async () => {
    const repositoryId = "00000000-0000-4000-8000-000000000010";
    let liveCalls = 0;
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.includes("/worktrees/live")) {
        liveCalls++;
        return jsonResponse({ error: "open repository: path missing" }, { status: 500 });
      }
      if (url.includes("/branches")) {
        return jsonResponse({ branches: [] });
      }
      return new Response("not found", { status: 404 });
    });

    const onReconcile = vi.fn();
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 } },
    });

    render(
      <QueryClientProvider client={client}>
        <RegisterWorktreeModal
          open
          pending={false}
          error={null}
          repositoryId={repositoryId}
          storedPath="/stale/old-checkout"
          onReconcile={onReconcile}
          onClose={() => {}}
          onSubmit={() => {}}
        />
      </QueryClientProvider>,
    );

    await waitFor(() => expect(onReconcile).toHaveBeenCalledTimes(1));
    await client.invalidateQueries({
      queryKey: gitQueryKeys.globalLiveWorktrees(repositoryId),
    });
    await waitFor(() => expect(liveCalls).toBeGreaterThan(1));
    expect(onReconcile).toHaveBeenCalledTimes(1);

    fetchMock.mockRestore();
  });

  it("lists the main worktree when live inventory marks it unregistered", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.includes("/worktrees/live")) {
        return jsonResponse({
          worktrees: [
            {
              path: "C:/repo/main",
              branch: "main",
              is_main: true,
              detached: false,
              registered: false,
              locked: false,
              prunable: false,
            },
          ],
        });
      }
      if (url.includes("/branches")) {
        return jsonResponse({ branches: [] });
      }
      return new Response("not found", { status: 404 });
    });

    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 } },
    });

    render(
      <QueryClientProvider client={client}>
        <RegisterWorktreeModal
          open
          pending={false}
          error={null}
          repositoryId="00000000-0000-4000-8000-000000000010"
          storedPath="C:/repo/main"
          onReconcile={vi.fn()}
          onClose={() => {}}
          onSubmit={() => {}}
        />
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByRole("combobox", { name: /worktree path/i })).toHaveTextContent(
        /Select a linked worktree/i,
      );
    });
    expect(screen.queryByText(/No unregistered worktrees/i)).not.toBeInTheDocument();

    fetchMock.mockRestore();
  });
});
