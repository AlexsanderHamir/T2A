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
    expect(document.querySelector(".draft-resume-state--loading")).not.toBeNull();
  });

  it("renders empty state when there are no drafts", () => {
    render(<DraftResumeModal {...baseProps()} />);
    expect(screen.getByRole("status")).toHaveTextContent(/no saved drafts yet/i);
    expect(document.querySelector(".draft-resume-state--empty")).not.toBeNull();
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
    expect(document.querySelector(".draft-resume-state--ready")).not.toBeNull();

    await user.click(screen.getByRole("button", { name: /resume: my draft/i }));
    expect(props.onResume).toHaveBeenCalledWith("d1");
    expect(screen.getByText(/page 1 of 1/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^previous$/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /^next$/i })).toBeDisabled();
  });

  it("renders a paginated scrollable list of drafts", async () => {
    const user = userEvent.setup();
    const props = {
      ...baseProps(),
      drafts: Array.from({ length: 7 }, (_, i) => ({
        id: `d${i + 1}`,
        name: `Draft ${i + 1}`,
        created_at: "",
        updated_at: "",
      })),
    };
    render(<DraftResumeModal {...props} />);

    expect(document.querySelector(".draft-resume-list")).not.toBeNull();
    expect(screen.getByText(/page 1 of 2/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /resume: draft 1/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /resume: draft 6/i })).toBeNull();

    await user.click(screen.getByRole("button", { name: /^next$/i }));
    expect(screen.getByText(/page 2 of 2/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /resume: draft 6/i })).toBeInTheDocument();
  });
});
