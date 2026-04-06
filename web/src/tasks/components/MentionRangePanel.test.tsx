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
    await waitFor(() => expect(fetchRepoFile).toHaveBeenCalledWith("src/foo.go"));
    await screen.findByLabelText(/preview/i);

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

    const ta = (await screen.findByLabelText(
      /preview/i,
    )) as HTMLTextAreaElement;
    ta.focus();
    ta.setSelectionRange(0, 5);
    fireEvent.select(ta);

    await user.click(screen.getByRole("button", { name: /insert selected range/i }));
    expect(onInsertWithRange).toHaveBeenCalledWith(1, 1);
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

    await screen.findByLabelText(/preview/i);
    expect(screen.getByRole("alert")).toHaveTextContent("Bad range");
  });
});
