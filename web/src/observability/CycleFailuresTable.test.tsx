import { render, screen } from "@testing-library/react";
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
  it("truncates long failure reasons and puts the full text in title", () => {
    const longReason =
      "Cursor account usage limit reached for the current model. Switch to another model in Settings, adjust Spend Limit in the Cursor app, or wait until your usage window resets.";
    const { container } = renderTable({
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

    expect(screen.queryByRole("button", { name: /read more/i })).toBeNull();
    const span = container.querySelector(".obs-failures-reason-text");
    expect(span).toBeTruthy();
    expect(span).toHaveAttribute("title", longReason);
    expect(span?.textContent).not.toBe(longReason);
  });

  it("shows short reasons in full without a title tooltip", () => {
    const { container } = renderTable({
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
    const span = container.querySelector(".obs-failures-reason-text");
    expect(span).toHaveTextContent("short");
    expect(span).not.toHaveAttribute("title");
  });
});
