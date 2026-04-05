import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { useState } from "react";
import { Modal } from "./Modal";

function Harness({
  onOpenChange,
}: {
  onOpenChange?: (open: boolean) => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        Open
      </button>
      {open ? (
        <Modal
          labelledBy="t-modal-title"
          onClose={() => {
            setOpen(false);
            onOpenChange?.(false);
          }}
        >
          <div>
            <h2 id="t-modal-title">Test</h2>
            <button type="button">First</button>
            <button type="button">Second</button>
          </div>
        </Modal>
      ) : null}
    </>
  );
}

describe("Modal", () => {
  it("moves focus to the first focusable control when opened", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: /^open$/i }));
    const first = await screen.findByRole("button", { name: /^first$/i });
    await waitFor(() => {
      expect(first).toHaveFocus();
    });
  });

  it("cycles Tab within the dialog", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: /^open$/i }));
    const first = await screen.findByRole("button", { name: /^first$/i });
    const second = screen.getByRole("button", { name: /^second$/i });
    await waitFor(() => expect(first).toHaveFocus());
    await user.tab();
    expect(second).toHaveFocus();
    await user.tab();
    expect(first).toHaveFocus();
  });

  it("restores focus to the trigger after close", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const openBtn = screen.getByRole("button", { name: /^open$/i });
    await user.click(openBtn);
    await screen.findByRole("dialog");
    await user.keyboard("{Escape}");
    await waitFor(() => {
      expect(openBtn).toHaveFocus();
    });
  });
});
