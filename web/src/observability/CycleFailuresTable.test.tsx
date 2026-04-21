import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { CycleFailuresTable } from "./CycleFailuresTable";

function renderTable(
  props: ComponentProps<typeof CycleFailuresTable>,
) {
  return render(
    <MemoryRouter>
      <CycleFailuresTable {...props} />
    </MemoryRouter>,
  );
}

describe("CycleFailuresTable", () => {
  it("truncates long failure reasons with read more / show less", async () => {
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

    expect(
      screen.getByRole("button", { name: /read more/i }),
    ).toBeInTheDocument();
    expect(screen.queryByText(longReason)).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /read more/i }));
    expect(screen.getByText(longReason)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /show less/i }));
    expect(screen.queryByText(longReason)).not.toBeInTheDocument();
  });

  it("does not show read more for short reasons", () => {
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
    expect(screen.queryByRole("button", { name: /read more/i })).toBeNull();
    expect(screen.getByText("short")).toBeInTheDocument();
  });
});
