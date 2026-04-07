import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { DraftResumeModal } from "./DraftResumeModal";

function baseProps() {
  return {
    drafts: [],
    onStartFresh: vi.fn(),
    onResume: vi.fn(),
    onClose: vi.fn(),
  };
}

describe("DraftResumeModal", () => {
  it("renders loading state while draft list is pending", () => {
    render(<DraftResumeModal {...baseProps()} loading />);
    expect(screen.getByRole("status")).toHaveTextContent(/loading drafts/i);
  });

  it("renders empty state when there are no drafts", () => {
    render(<DraftResumeModal {...baseProps()} />);
    expect(screen.getByRole("status")).toHaveTextContent(/no saved drafts yet/i);
  });

  it("renders error state and start fresh remains actionable", async () => {
    const user = userEvent.setup();
    const props = baseProps();
    render(<DraftResumeModal {...props} loadError="drafts unavailable" />);

    expect(screen.getByRole("alert")).toHaveTextContent(/drafts unavailable/i);
    await user.click(screen.getByRole("button", { name: /^start fresh$/i }));
    expect(props.onStartFresh).toHaveBeenCalledTimes(1);
  });

  it("renders retry action for draft list errors", async () => {
    const user = userEvent.setup();
    const props = { ...baseProps(), onRetryLoad: vi.fn() };
    render(<DraftResumeModal {...props} loadError="drafts unavailable" />);

    await user.click(screen.getByRole("button", { name: /retry loading drafts/i }));
    expect(props.onRetryLoad).toHaveBeenCalledTimes(1);
  });

  it("renders draft actions and resumes selected draft", async () => {
    const user = userEvent.setup();
    const props = {
      ...baseProps(),
      drafts: [{ id: "d1", name: "My draft", created_at: "", updated_at: "" }],
    };
    render(<DraftResumeModal {...props} />);

    await user.click(screen.getByRole("button", { name: /resume: my draft/i }));
    expect(props.onResume).toHaveBeenCalledWith("d1");
  });
});
