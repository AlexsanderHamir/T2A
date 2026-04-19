import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MutationErrorBanner } from "./MutationErrorBanner";

describe("MutationErrorBanner", () => {
  it("renders nothing when error is null", () => {
    // Steady-state idle: callers pass `mutation.error` which is null
    // until the mutation settles into an error. The banner must not
    // emit an empty `role="alert"` live-region or screen-readers will
    // see flicker on every render of the parent component.
    const { container } = render(<MutationErrorBanner error={null} />);
    expect(container.firstChild).toBeNull();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("renders nothing when error is undefined", () => {
    // Same contract as null — callers may pass a sometimes-undefined
    // value (e.g. when the prop is optional in the parent) and should
    // not have to gate on `error ? ... : null` themselves.
    const { container } = render(<MutationErrorBanner error={undefined} />);
    expect(container.firstChild).toBeNull();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("renders an Error instance's .message inside an alert region", () => {
    // The react-query case: `mutation.error` is `Error | null` (per
    // their v5 typing). The banner must surface the underlying
    // `.message` verbatim so the user sees what the server reported.
    render(<MutationErrorBanner error={new Error("server returned 500")} />);
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(/server returned 500/i);
  });

  it("renders a pre-coerced string verbatim (does NOT route through errorMessage's fallback path)", () => {
    // Critical contract: the flow hooks (`useTaskPatchFlow`,
    // `useTaskDeleteFlow`) already pre-coerce via
    // `errorMessage(mutation.error)` before exposing `patchError` /
    // `deleteError`, so by the time the string reaches the banner it
    // is already user-presentable. Routing it back through
    // `errorMessage(error, fallback)` would lose the string and pick
    // up the fallback (because `String("...") === "..."` but
    // `fallback ?? String(...)` short-circuits on the truthy fallback
    // before checking the bare string). This test pins the special
    // case so a future refactor doesn't accidentally re-introduce
    // the double-coercion bug.
    render(
      <MutationErrorBanner
        error="title cannot be empty"
        fallback="Could not save changes."
      />,
    );
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(/title cannot be empty/i);
    expect(alert).not.toHaveTextContent(/could not save changes/i);
  });

  it("uses the fallback for non-Error / non-string throwables", () => {
    // Defensive against future flows that throw a plain object or
    // similar — the banner should never render `[object Object]`.
    // Same contract as `errorMessage(value, fallback)` — pinned by
    // `errorMessage.test.ts` separately, this test just verifies the
    // banner forwards through.
    render(
      <MutationErrorBanner
        error={{ unexpected: true } as unknown as Error}
        fallback="Could not complete request."
      />,
    );
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(/could not complete request/i);
  });

  it("appends a custom className alongside the canonical 'err' class", () => {
    // Per-site visual hooks (e.g. `task-create-modal-err--evaluate`
    // for positioning the callout next to the evaluate action). The
    // canonical `err` selector must always be present so the
    // app-base styling (`.err > p { margin: 0 }`, danger background)
    // applies everywhere.
    render(
      <MutationErrorBanner
        error={new Error("boom")}
        className="task-create-modal-err--evaluate"
      />,
    );
    const alert = screen.getByRole("alert");
    expect(alert).toHaveClass("err");
    expect(alert).toHaveClass("task-create-modal-err--evaluate");
  });

  it("renders only the canonical 'err' class when no className is provided", () => {
    // Pin: extra whitespace / leading space bugs from `'err ' +
    // (className ?? '')`-style concatenation must not leak into the
    // class list. Use `getAttribute('class')` so we can assert the
    // exact serialization without classList normalization.
    render(<MutationErrorBanner error={new Error("boom")} />);
    const alert = screen.getByRole("alert");
    expect(alert.getAttribute("class")).toBe("err");
  });
});
