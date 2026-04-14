import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { AppErrorBoundary } from "./AppErrorBoundary";

function CrashOnRender(): JSX.Element {
  throw new Error("boom");
}

function RecoverAfterReset() {
  const [bad, setBad] = useState(true);
  return (
    <AppErrorBoundary onRecover={() => setBad(false)}>
      {bad ? <CrashOnRender /> : <div>Recovered content</div>}
    </AppErrorBoundary>
  );
}

function findScopedBoundaryLogMessage(calls: unknown[][]): string | undefined {
  for (const c of calls) {
    const first = c[0];
    if (
      typeof first === "string" &&
      first.includes("[AppErrorBoundary:")
    ) {
      return first;
    }
  }
  return undefined;
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

  it("renders custom fallback message when provided", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <AppErrorBoundary fallbackMessage="Custom crash note.">
        <CrashOnRender />
      </AppErrorBoundary>,
    );

    expect(screen.getByRole("alert")).toHaveTextContent("Custom crash note.");
    const customMsg = findScopedBoundaryLogMessage(errorSpy.mock.calls);
    expect(customMsg).toBeDefined();
    expect(customMsg).toContain("[AppErrorBoundary:app-root]");
    errorSpy.mockRestore();
  });

  it("logs route-outlet scope when variant is route-outlet", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <AppErrorBoundary variant="route-outlet">
        <CrashOnRender />
      </AppErrorBoundary>,
    );

    const routeMsg = findScopedBoundaryLogMessage(errorSpy.mock.calls);
    expect(routeMsg).toBeDefined();
    expect(routeMsg).toContain("[AppErrorBoundary:route-outlet]");
    errorSpy.mockRestore();
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
      within(alert).getByRole("button", { name: /^try again$/i }),
    ).toBeInTheDocument();
    expect(
      within(alert).getByRole("button", { name: /^reload page$/i }),
    ).toBeInTheDocument();
    errorSpy.mockRestore();
  });

  it("invokes onRecover when Try again is pressed", async () => {
    const user = userEvent.setup();
    const onRecover = vi.fn();
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <AppErrorBoundary onRecover={onRecover}>
        <CrashOnRender />
      </AppErrorBoundary>,
    );

    await user.click(screen.getByRole("button", { name: /^try again$/i }));
    expect(onRecover).toHaveBeenCalledTimes(1);
    errorSpy.mockRestore();
  });

  it("Try again shows children again when onRecover makes the tree safe", async () => {
    const user = userEvent.setup();
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    render(<RecoverAfterReset />);

    expect(screen.getByRole("alert")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /^try again$/i }));
    expect(await screen.findByText("Recovered content")).toBeInTheDocument();
    errorSpy.mockRestore();
  });
});
