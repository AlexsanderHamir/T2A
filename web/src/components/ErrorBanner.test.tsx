import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ErrorBanner } from "./ErrorBanner";

describe("ErrorBanner", () => {
  it("renders as an alert with the message", () => {
    render(<ErrorBanner message="Something went wrong" />);
    const el = screen.getByRole("alert");
    expect(el).toHaveTextContent("Something went wrong");
  });
});
