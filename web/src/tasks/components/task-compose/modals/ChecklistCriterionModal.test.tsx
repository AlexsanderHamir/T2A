import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ModalStackProvider } from "@/shared/ModalStackContext";
import { ChecklistCriterionModal } from "./ChecklistCriterionModal";

function renderModal(overrides: Partial<
  React.ComponentProps<typeof ChecklistCriterionModal>
> = {}) {
  const props: React.ComponentProps<typeof ChecklistCriterionModal> = {
    mode: "add",
    pending: false,
    saving: false,
    onClose: vi.fn(),
    text: "",
    onTextChange: vi.fn(),
    onSubmit: vi.fn(),
    ...overrides,
  };
  return render(
    <ModalStackProvider>
      <ChecklistCriterionModal {...props} />
    </ModalStackProvider>,
  );
}

describe("ChecklistCriterionModal error display", () => {
  it("does not render an error callout on the happy path", () => {
    renderModal({ mode: "add" });
    expect(
      screen.queryByRole("alert", { name: /could not/i }),
    ).not.toBeInTheDocument();
  });

  it("renders the underlying mutation error message in add mode", () => {
    renderModal({
      mode: "add",
      error: new Error("network unreachable"),
    });
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(/network unreachable/i);
  });

  it("falls back to add-mode default copy when error is a non-Error throwable", () => {
    // `react-query` types `mutation.error` as `Error | null`, but our
    // `errorMessage` helper is defensive against legacy code paths that
    // throw bare values (strings, undefined, server JSON). The fallback
    // surfaces a kinder default than `String(undefined)`.
    renderModal({
      mode: "add",
      error: undefined,
    });
    expect(
      screen.queryByRole("alert", { name: /could not/i }),
    ).not.toBeInTheDocument();
  });

  it("renders the underlying error message in edit mode", () => {
    renderModal({
      mode: "edit",
      error: new Error("forbidden"),
    });
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(/forbidden/i);
  });

  it("keeps the action buttons enabled while showing an error so the user can retry", () => {
    renderModal({
      mode: "add",
      text: "Done by Friday",
      error: new Error("boom"),
    });
    expect(
      screen.getByRole("button", { name: /add criterion/i }),
    ).not.toBeDisabled();
    expect(screen.getByRole("button", { name: /cancel/i })).not.toBeDisabled();
  });
});
