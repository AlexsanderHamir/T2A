import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { CycleFailuresTable } from "./CycleFailuresTable";

function renderTable(props: ComponentProps<typeof CycleFailuresTable>) {
  return render(
    <MemoryRouter>
      <CycleFailuresTable {...props} />
    </MemoryRouter>,
  );
}

describe("CycleFailuresTable", () => {
  it("offers show more / show less for long failure reasons", async () => {
    const user = userEvent.setup();
    const longReason =
      "Cursor account usage limit reached for the current model. Switch to another model in Settings, adjust Spend Limit in the Cursor app, or wait until your usage window resets.";
    renderTable({
      failures: [
        {
          task_id: "t1",
          event_seq: 1,
          at: "2026-04-20T12:00:00Z",
          cycle_id: "c1",
          attempt_seq: 1,
          status: "failed",
          reason: longReason,
        },
      ],
    });

    const showMore = screen.getByRole("button", { name: /^show more$/i });
    expect(showMore).toHaveAttribute("aria-expanded", "false");
    const textEl = screen.getByText(longReason, { exact: false });
    expect(textEl.tagName).toBe("P");

    await user.click(showMore);
    expect(screen.getByRole("button", { name: /^show less$/i })).toHaveAttribute(
      "aria-expanded",
      "true",
    );
    expect(screen.getByText(longReason)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /^show less$/i }));
    expect(screen.getByRole("button", { name: /^show more$/i })).toBeInTheDocument();
  });

  it("shows short reasons in full with no disclosure control", () => {
    renderTable({
      failures: [
        {
          task_id: "t1",
          event_seq: 1,
          at: "2026-04-20T12:00:00Z",
          cycle_id: "c1",
          attempt_seq: 1,
          status: "failed",
          reason: "short",
        },
      ],
    });
    expect(screen.getByText("short")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /show more/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /show less/i }),
    ).not.toBeInTheDocument();
  });
});
