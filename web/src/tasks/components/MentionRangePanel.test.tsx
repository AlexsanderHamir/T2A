import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { MentionRangePanel } from "./MentionRangePanel";

describe("MentionRangePanel", () => {
  it("shows path and calls actions", async () => {
    const user = userEvent.setup();
    const onInsertPathOnly = vi.fn();
    const onCancel = vi.fn();

    render(
      <MentionRangePanel
        id="p1"
        path="src/foo.go"
        lineStart=""
        lineEnd=""
        rangeWarning={null}
        onLineStartChange={vi.fn()}
        onLineEndChange={vi.fn()}
        onInsertWithRange={vi.fn()}
        onInsertPathOnly={onInsertPathOnly}
        onCancel={onCancel}
      />,
    );

    expect(screen.getByText("src/foo.go")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /insert file only/i }));
    expect(onInsertPathOnly).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: /^cancel$/i }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("shows range warning", () => {
    render(
      <MentionRangePanel
        id="p2"
        path="x"
        lineStart="1"
        lineEnd="2"
        rangeWarning="Bad range"
        onLineStartChange={vi.fn()}
        onLineEndChange={vi.fn()}
        onInsertWithRange={vi.fn()}
        onInsertPathOnly={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Bad range");
  });
});
