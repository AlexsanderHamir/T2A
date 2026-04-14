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
