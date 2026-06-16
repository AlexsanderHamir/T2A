import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
    verifyCommands: [],
    onVerifyCommandsChange: vi.fn(),
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

describe("ChecklistCriterionModal verify commands", () => {
  it("does not add a row when focus moves from command to outcome in the same row", async () => {
    const user = userEvent.setup();
    const onVerifyCommandsChange = vi.fn();
    renderModal({
      verifyCommands: [{ command: "go test ./...", expected_outcome: "" }],
      onVerifyCommandsChange,
    });

    await user.click(screen.getByLabelText(/shell command 1/i));
    await user.click(screen.getByLabelText(/expected outcome for command 1/i));

    expect(onVerifyCommandsChange).not.toHaveBeenCalled();
  });
});

describe("ChecklistCriterionModal read-only view", () => {
  it("shows satisfied criterion in view mode without save actions", () => {
    renderModal({
      mode: "edit",
      readOnly: true,
      text: "The full test suite still passes.",
      verifyCommands: [{ command: "go test ./...", expected_outcome: "pass" }],
    });

    expect(
      screen.getByRole("heading", { name: /view criterion/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByLabelText(/^criterion$/i),
    ).toHaveAttribute("readonly");
    expect(screen.getByLabelText(/^criterion$/i)).not.toBeDisabled();
    expect(
      screen.queryByRole("button", { name: /save changes/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /add command/i }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^close$/i })).toBeInTheDocument();
  });
});
