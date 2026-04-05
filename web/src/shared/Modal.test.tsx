import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { useState } from "react";
import { Modal } from "./Modal";
import { ModalStackProvider } from "./ModalStackContext";

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
          describedBy="t-modal-desc"
          onClose={() => {
            setOpen(false);
            onOpenChange?.(false);
          }}
        >
          <div>
            <h2 id="t-modal-title">Test</h2>
            <p id="t-modal-desc">
              Extra context for assistive technology.
            </p>
            <button type="button">First</button>
            <button type="button">Second</button>
          </div>
        </Modal>
      ) : null}
    </>
  );
}

function NestedStackHarness() {
  const [outer, setOuter] = useState(false);
  const [inner, setInner] = useState(false);
  return (
    <ModalStackProvider>
      <button type="button" onClick={() => setOuter(true)}>
        Open outer
      </button>
      {outer ? (
        <Modal
          labelledBy="outer-modal-title"
          onClose={() => {
            setInner(false);
            setOuter(false);
          }}
        >
          <div>
            <h2 id="outer-modal-title">Outer</h2>
            <button type="button" onClick={() => setInner(true)}>
              Open inner
            </button>
          </div>
          {inner ? (
            <Modal
              labelledBy="inner-modal-title"
              onClose={() => setInner(false)}
              stack="nested"
              lockBodyScroll={false}
            >
              <div>
                <h2 id="inner-modal-title">Inner</h2>
              </div>
            </Modal>
          ) : null}
        </Modal>
      ) : null}
    </ModalStackProvider>
  );
}

describe("Modal", () => {
  it("exposes aria-describedby when describedBy is set", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: /^open$/i }));
    const dialog = await screen.findByRole("dialog", {
      description: /extra context for assistive technology/i,
    });
    expect(dialog).toHaveAttribute("aria-describedby", "t-modal-desc");
  });

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

  it("sets inert on #root while a modal is open under ModalStackProvider", async () => {
    const user = userEvent.setup();
    const rootHost = document.createElement("div");
    rootHost.id = "root";
    document.body.appendChild(rootHost);
    try {
      render(
        <ModalStackProvider>
          <Harness />
        </ModalStackProvider>,
        { container: rootHost },
      );
      expect(rootHost.hasAttribute("inert")).toBe(false);
      await user.click(screen.getByRole("button", { name: /^open$/i }));
      await screen.findByRole("dialog");
      expect(rootHost.hasAttribute("inert")).toBe(true);
      await user.keyboard("{Escape}");
      await waitFor(() => {
        expect(rootHost.hasAttribute("inert")).toBe(false);
      });
    } finally {
      rootHost.remove();
    }
  });

  it("closes only the top modal on Escape when stacked under ModalStackProvider", async () => {
    const user = userEvent.setup();
    render(<NestedStackHarness />);
    await user.click(screen.getByRole("button", { name: /open outer/i }));
    await screen.findByRole("heading", { name: /^outer$/i });
    await user.click(screen.getByRole("button", { name: /open inner/i }));
    await screen.findByRole("heading", { name: /^inner$/i });

    await user.keyboard("{Escape}");
    await waitFor(() => {
      expect(screen.queryByRole("heading", { name: /^inner$/i })).toBeNull();
    });
    expect(screen.getByRole("heading", { name: /^outer$/i })).toBeInTheDocument();

    await user.keyboard("{Escape}");
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
  });
});
