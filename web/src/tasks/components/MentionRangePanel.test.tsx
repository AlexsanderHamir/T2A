import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fetchRepoFile } from "@/api/repo";
import { MentionRangePanel } from "./MentionRangePanel";

vi.mock("@/api/repo", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api/repo")>();
  return { ...actual, fetchRepoFile: vi.fn() };
});

const sampleFile = {
  path: "src/foo.go",
  content: "hello\nworld\n",
  binary: false,
  truncated: false,
  size_bytes: 12,
  line_count: 2,
};

describe("MentionRangePanel", () => {
  beforeEach(() => {
    vi.mocked(fetchRepoFile).mockResolvedValue(sampleFile);
  });

  it("loads preview and calls insert file only", async () => {
    const user = userEvent.setup();
    const onInsertPathOnly = vi.fn();
    const onCancel = vi.fn();

    render(
      <MentionRangePanel
        id="p1"
        path="src/foo.go"
        rangeWarning={null}
        onInsertWithRange={vi.fn()}
        onInsertPathOnly={onInsertPathOnly}
        onCancel={onCancel}
      />,
    );

    expect(screen.getByText("src/foo.go")).toBeInTheDocument();
    await waitFor(() =>
      expect(fetchRepoFile).toHaveBeenCalledWith(
        "src/foo.go",
        expect.objectContaining({ signal: expect.any(AbortSignal) }),
      ),
    );
    await screen.findByRole("textbox", { name: /preview/i });

    await user.click(screen.getByRole("button", { name: /insert file only/i }));
    expect(onInsertPathOnly).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: /^cancel$/i }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("inserts selected range when text is highlighted", async () => {
    const user = userEvent.setup();
    const onInsertWithRange = vi.fn();

    render(
      <MentionRangePanel
        id="p2"
        path="src/foo.go"
        rangeWarning={null}
        onInsertWithRange={onInsertWithRange}
        onInsertPathOnly={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    const ta = (await screen.findByRole("textbox", {
      name: /preview/i,
    })) as HTMLTextAreaElement;
    ta.focus();
    ta.setSelectionRange(0, 5);
    fireEvent.select(ta);

    await user.click(screen.getByRole("button", { name: /insert line range/i }));
    expect(onInsertWithRange).toHaveBeenCalledWith(1, 1);
  });

  it("inserts manual line range when typed", async () => {
    const user = userEvent.setup();
    const onInsertWithRange = vi.fn();

    render(
      <MentionRangePanel
        id="p4"
        path="src/foo.go"
        rangeWarning={null}
        onInsertWithRange={onInsertWithRange}
        onInsertPathOnly={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    await screen.findByRole("textbox", { name: /preview/i });
    await user.type(screen.getByLabelText(/from line/i), "1");
    await user.type(screen.getByLabelText(/to line/i), "2");

    await user.click(screen.getByRole("button", { name: /insert line range/i }));
    expect(onInsertWithRange).toHaveBeenCalledWith(1, 2);
  });

  it("shows range warning", async () => {
    render(
      <MentionRangePanel
        id="p3"
        path="x"
        rangeWarning="Bad range"
        onInsertWithRange={vi.fn()}
        onInsertPathOnly={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    await screen.findByRole("textbox", { name: /preview/i });
    expect(screen.getByRole("alert")).toHaveTextContent("Bad range");
  });

  it("shows loading preview skeleton then file content", async () => {
    let resolveFile!: (value: typeof sampleFile) => void;
    const deferred = new Promise<typeof sampleFile>((resolve) => {
      resolveFile = resolve;
    });
    vi.mocked(fetchRepoFile).mockReturnValue(
      deferred as Promise<(typeof sampleFile) | null>,
    );

    render(
      <MentionRangePanel
        id="p-load"
        path="src/foo.go"
        rangeWarning={null}
        onInsertWithRange={vi.fn()}
        onInsertPathOnly={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("status", { name: /loading file from repository/i }),
    ).toBeInTheDocument();
    resolveFile(sampleFile);
    await screen.findByRole("textbox", { name: /preview/i });
  });

  it("shows load error with try again and refetches", async () => {
    const user = userEvent.setup();
    let calls = 0;
    vi.mocked(fetchRepoFile).mockImplementation(async (p, init) => {
      expect(p).toBe("src/foo.go");
      expect(init).toEqual(expect.objectContaining({ signal: expect.any(AbortSignal) }));
      calls += 1;
      if (calls === 1) {
        throw new Error("offline");
      }
      return sampleFile;
    });

    render(
      <MentionRangePanel
        id="p-retry"
        path="src/foo.go"
        rangeWarning={null}
        onInsertWithRange={vi.fn()}
        onInsertPathOnly={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(/offline/i);
    await user.click(screen.getByRole("button", { name: /try again/i }));
    await screen.findByRole("textbox", { name: /preview/i });
    expect(calls).toBe(2);
  });

  it("shows an error when insert line range fails", async () => {
    const user = userEvent.setup();
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const onInsertWithRange = vi.fn().mockRejectedValue(new Error("Network down"));

    render(
      <MentionRangePanel
        id="p5"
        path="src/foo.go"
        rangeWarning={null}
        onInsertWithRange={onInsertWithRange}
        onInsertPathOnly={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    await screen.findByRole("textbox", { name: /preview/i });
    await user.type(screen.getByLabelText(/from line/i), "1");
    await user.type(screen.getByLabelText(/to line/i), "2");

    await user.click(screen.getByRole("button", { name: /insert line range/i }));

    expect(await screen.findByText("Network down")).toBeInTheDocument();
    errorSpy.mockRestore();
  });
});
