import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { ErrorBanner } from "./ErrorBanner";

describe("ErrorBanner", () => {
  it("renders as an alert with the message", () => {
    render(<ErrorBanner message="Something went wrong" />);
    const el = screen.getByRole("alert");
    expect(el).toHaveTextContent("Something went wrong");
  });

  it("hides when dismissed and reappears when the message changes", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<ErrorBanner message="First error" />);
    expect(screen.getByRole("alert")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /dismiss error/i }));
    expect(screen.queryByRole("alert")).toBeNull();

    rerender(<ErrorBanner message="Second error" />);
    expect(screen.getByRole("alert")).toHaveTextContent("Second error");
  });
});
