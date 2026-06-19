import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { CycleCommit } from "@/types";
import { CommitList } from "./CommitList";

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

function createWrapper() {
  const qc = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, staleTime: 0 },
    },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
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

  it("lazy-loads diff on row expansion", async () => {
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

    const user = userEvent.setup();
    const Wrapper = createWrapper();
    render(
      <Wrapper>
        <CommitList commits={sampleCommits} />
      </Wrapper>,
    );

    expect(diffCalls).toHaveLength(0);

    const summary = screen.getByText("refactor(web): split helpers").closest(
      "summary",
    );
    expect(summary).not.toBeNull();
    await user.click(summary!);

    await waitFor(() => {
      expect(diffCalls.length).toBeGreaterThanOrEqual(1);
    });
    expect(diffCalls[0]).toContain("/repo/diff?sha=");
    await screen.findByText("note.txt");
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
      throw new Error(`unexpected fetch ${url}`);
    });

    const user = userEvent.setup();
    const Wrapper = createWrapper();
    render(
      <Wrapper>
        <CommitList commits={sampleCommits} />
      </Wrapper>,
    );

    await user.click(screen.getByText("refactor(web): split helpers"));

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

  it("shows truncation banner and full diff action when truncated", async () => {
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
      throw new Error(`unexpected fetch ${url}`);
    });

    const user = userEvent.setup();
    const Wrapper = createWrapper();
    render(
      <Wrapper>
        <CommitList commits={sampleCommits} />
      </Wrapper>,
    );

    await user.click(screen.getByText("refactor(web): split helpers"));

    expect(
      await screen.findByText(/Diff preview is truncated/i),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /view full diff/i }),
    ).toBeInTheDocument();
  });
});
