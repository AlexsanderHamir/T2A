import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AppErrorBoundary } from "./AppErrorBoundary";

function CrashOnRender(): JSX.Element {
  throw new Error("boom");
}

describe("AppErrorBoundary", () => {
  it("renders children when there is no render error", () => {
    render(
      <AppErrorBoundary>
        <div>Safe content</div>
      </AppErrorBoundary>,
    );

    expect(screen.getByText("Safe content")).toBeInTheDocument();
  });

  it("renders fallback UI when child render throws", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <AppErrorBoundary>
        <CrashOnRender />
      </AppErrorBoundary>,
    );

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(
      "Something went wrong while rendering this page.",
    );
    expect(
      within(alert).getByRole("button", { name: /^reload page$/i }),
    ).toBeInTheDocument();
    errorSpy.mockRestore();
  });
});
