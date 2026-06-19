import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import type { CycleCommit } from "@/types";
import { CommitList } from "./CommitList";
import { TaskCommitDiffPage } from "@/tasks/pages/TaskCommitDiffPage";

const taskId = "task-1";
const sampleCommits: CycleCommit[] = [
  {
    seq: 1,
    repo: "/repo",
    worktree: "/repo",
    branch: "main",
    sha: "abc1234567890abcdef1234567890abcdef1234",
    committed_at: "2026-04-18T10:00:00.000Z",
    message: "refactor(web): split helpers",
    status: "eligible",
  },
];

function createWrapper(initialEntries = ["/"]) {
  const qc = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, staleTime: 0 },
    },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={qc}>
        <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={initialEntries}>
          <Routes>
            <Route path="/" element={<>{children}</>} />
            <Route
              path="/tasks/:taskId/commits/:sha"
              element={<TaskCommitDiffPage />}
            />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    );
  };
}

const okJSON = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });

const samplePatch = [
  "diff --git a/note.txt b/note.txt",
  "index 83db48f..f00f10f 100644",
  "--- a/note.txt",
  "+++ b/note.txt",
  "@@ -1 +1 @@",
  "-hello",
  "+world",
].join("\n");

describe("CommitList", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("does not fetch diff until the commit row is opened", async () => {
    const diffCalls: string[] = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      if (url.includes("/repo/diff")) {
        diffCalls.push(url);
        return okJSON({
          sha: sampleCommits[0].sha,
          patch: samplePatch,
          truncated: false,
          size_bytes: samplePatch.length,
        });
      }
      throw new Error(`unexpected fetch ${url}`);
    });

    const Wrapper = createWrapper();
    render(
      <Wrapper>
        <CommitList taskId={taskId} commits={sampleCommits} />
      </Wrapper>,
    );

    expect(diffCalls).toHaveLength(0);
    expect(
      screen.getByRole("link", {
        name: /view diff for abc1234: refactor\(web\): split helpers/i,
      }),
    ).toHaveAttribute(
      "href",
      `/tasks/${encodeURIComponent(taskId)}/commits/${encodeURIComponent(sampleCommits[0].sha)}`,
    );
  });

  it("navigates to the commit diff page and loads the patch", async () => {
    const diffCalls: string[] = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      if (url.includes("/repo/diff")) {
        diffCalls.push(url);
        return okJSON({
          sha: sampleCommits[0].sha,
          patch: samplePatch,
          truncated: false,
          size_bytes: samplePatch.length,
        });
      }
      if (url.includes(`/tasks/${taskId}/commits`) && !url.includes("/repo/")) {
        return okJSON({ commits: sampleCommits });
      }
      throw new Error(`unexpected fetch ${url}`);
    });

    const user = userEvent.setup();
    const Wrapper = createWrapper();
    render(
      <Wrapper>
        <CommitList taskId={taskId} commits={sampleCommits} />
      </Wrapper>,
    );

    await user.click(
      screen.getByRole("link", {
        name: /view diff for abc1234: refactor\(web\): split helpers/i,
      }),
    );

    expect(await screen.findByTestId("task-commit-diff-page")).toBeInTheDocument();
    await waitFor(() => {
      expect(diffCalls.length).toBeGreaterThanOrEqual(1);
    });
    expect(diffCalls[0]).toContain("/repo/diff?sha=");
    await screen.findByText("note.txt");
  });
});

describe("TaskCommitDiffPage", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows error state and retries when diff fetch fails", async () => {
    let attempts = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      if (url.includes("/repo/diff")) {
        attempts += 1;
        return new Response(JSON.stringify({ error: "boom" }), {
          status: 500,
          headers: { "Content-Type": "application/json" },
        });
      }
      if (url.includes(`/tasks/${taskId}/commits`) && !url.includes("/repo/")) {
        return okJSON({ commits: sampleCommits });
      }
      throw new Error(`unexpected fetch ${url}`);
    });

    const user = userEvent.setup();
    const Wrapper = createWrapper([
      `/tasks/${taskId}/commits/${sampleCommits[0].sha}`,
    ]);
    render(<Wrapper>{null}</Wrapper>);

    await waitFor(
      () => {
        expect(screen.getByRole("alert")).toBeInTheDocument();
      },
      { timeout: 5000 },
    );
    const alert = screen.getByRole("alert");
    expect(within(alert).getByText(/Could not load diff|boom/i)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /try again/i }));
    await waitFor(
      () => {
        expect(attempts).toBeGreaterThanOrEqual(2);
      },
      { timeout: 5000 },
    );
  });

  it("shows truncation notice when the patch is truncated", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      if (url.includes("/repo/diff")) {
        return okJSON({
          sha: sampleCommits[0].sha,
          patch: samplePatch,
          truncated: true,
          size_bytes: samplePatch.length,
        });
      }
      if (url.includes(`/tasks/${taskId}/commits`) && !url.includes("/repo/")) {
        return okJSON({ commits: sampleCommits });
      }
      throw new Error(`unexpected fetch ${url}`);
    });

    const Wrapper = createWrapper([
      `/tasks/${taskId}/commits/${sampleCommits[0].sha}`,
    ]);
    render(<Wrapper>{null}</Wrapper>);

    expect(
      await screen.findByText(/truncated at the server limit/i),
    ).toBeInTheDocument();
  });
});
