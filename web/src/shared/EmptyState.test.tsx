import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { EmptyState } from "./EmptyState";

describe("EmptyState", () => {
  it("renders title and description", () => {
    render(
      <EmptyState
        title="Nothing here"
        description="Add something to get started."
      />,
    );
    expect(
      screen.getByRole("heading", { name: "Nothing here" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Add something to get started."),
    ).toBeInTheDocument();
  });

  it("invokes action on button click", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(
      <EmptyState
        title="Empty"
        description="Desc"
        action={{ label: "Go", onClick }}
      />,
    );
    await user.click(screen.getByRole("button", { name: /^go$/i }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});
