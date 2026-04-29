import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SchedulePicker } from "./SchedulePicker";

const NY = "America/New_York";
const TOKYO = "Asia/Tokyo";

// 2026-04-19T12:00:00Z = noon UTC, Sunday. Anchored against this clock so
// quick-pick offsets ("+1h", "+3d", "+1mo") are deterministic regardless of
// the host machine's locale or the picker's `appTimezone`.
const NOON_2026_04_19 = new Date("2026-04-19T12:00:00Z").getTime();

describe("SchedulePicker render", () => {
  it("renders empty input + 'immediately' caption when value is null", () => {
    render(
      <SchedulePicker value={null} onChange={() => {}} appTimezone="UTC" />,
    );
    const input = screen.getByTestId(
      "schedule-picker-input",
    ) as HTMLInputElement;
    expect(input.value).toBe("");
    expect(
      screen.getByText(/picks up immediately when the worker is free/i),
    ).toBeInTheDocument();
  });

  it("renders the wall-clock literal in the operator's timezone (NY)", () => {
    render(
      <SchedulePicker
        value="2026-04-19T13:00:00Z"
        onChange={() => {}}
        appTimezone={NY}
      />,
    );
    const input = screen.getByTestId(
      "schedule-picker-input",
    ) as HTMLInputElement;
    // 13:00Z = 09:00 EDT.
    expect(input.value).toBe("2026-04-19T09:00");
    // Caption shows the formatted instant in NY.
    expect(screen.getByText(/agent will pick up at/i)).toBeInTheDocument();
    expect(screen.getByText(/09:00/)).toBeInTheDocument();
  });

  it("renders the wall-clock literal in Asia/Tokyo (no DST, UTC+9)", () => {
    render(
      <SchedulePicker
        value="2026-04-19T00:00:00Z"
        onChange={() => {}}
        appTimezone={TOKYO}
      />,
    );
    const input = screen.getByTestId(
      "schedule-picker-input",
    ) as HTMLInputElement;
    expect(input.value).toBe("2026-04-19T09:00");
  });

  it("respects disabled prop on the fieldset (chips inherit via fieldset[disabled])", () => {
    render(
      <SchedulePicker
        value={null}
        onChange={() => {}}
        appTimezone="UTC"
        disabled
      />,
    );
    const fieldset = screen
      .getByTestId("schedule-picker-input")
      .closest("fieldset") as HTMLFieldSetElement;
    expect(fieldset.disabled).toBe(true);
  });

  it("does not show the offset popover by default", () => {
    render(
      <SchedulePicker value={null} onChange={() => {}} appTimezone="UTC" />,
    );
    expect(
      screen.queryByTestId("schedule-picker-quick-popover"),
    ).not.toBeInTheDocument();
  });
});

describe("SchedulePicker manual input", () => {
  it("emits a UTC ISO string when the operator types a wall-clock literal in NY", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker value={null} onChange={onChange} appTimezone={NY} />,
    );
    const input = screen.getByTestId(
      "schedule-picker-input",
    ) as HTMLInputElement;
    // <input type="datetime-local"> emits via change events; use
    // fireEvent-style direct value set + change so we don't rely on
    // browser native typing into a non-text input.
    await user.type(input, "2026-04-22T09:00");
    // `userEvent.type` against a datetime-local input fires a single
    // change with the assembled value in jsdom; the latest call
    // contains the final ISO.
    expect(onChange).toHaveBeenCalled();
    const last = onChange.mock.calls[onChange.mock.calls.length - 1][0];
    // 09:00 EDT on 2026-04-22 = 13:00 UTC.
    expect(last).toBe("2026-04-22T13:00:00.000Z");
  });

  it("emits null when the operator clears the input", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value="2026-04-22T13:00:00Z"
        onChange={onChange}
        appTimezone={NY}
      />,
    );
    const input = screen.getByTestId(
      "schedule-picker-input",
    ) as HTMLInputElement;
    await user.clear(input);
    expect(onChange).toHaveBeenCalledWith(null);
  });

  it("'Clear' icon emits null", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value="2026-04-22T13:00:00Z"
        onChange={onChange}
        appTimezone={NY}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-clear"));
    expect(onChange).toHaveBeenCalledWith(null);
  });
});

describe("SchedulePicker quick-pick popover", () => {
  it("opens the popover with sectioned offset chips when the trigger is clicked", async () => {
    const user = userEvent.setup();
    render(
      <SchedulePicker
        value={null}
        onChange={() => {}}
        appTimezone={NY}
        nowMs={NOON_2026_04_19}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-quick-trigger"));
    const popover = screen.getByTestId("schedule-picker-quick-popover");
    expect(popover).toBeInTheDocument();
    // Section headings are visible and labelled.
    expect(
      screen.getByRole("heading", { name: /^minutes$/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: /^hours$/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: /^days$/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: /^weeks$/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: /^months$/i }),
    ).toBeInTheDocument();
    // Sample chips are present.
    expect(
      screen.getByTestId("schedule-picker-quick-minute-10"),
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("schedule-picker-quick-hour-24"),
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("schedule-picker-quick-month-12"),
    ).toBeInTheDocument();
  });

  it("'+10 minutes' emits now + 10 minutes (timezone-agnostic)", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={NOON_2026_04_19}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-quick-trigger"));
    await user.click(screen.getByTestId("schedule-picker-quick-minute-10"));
    expect(onChange).toHaveBeenCalledWith("2026-04-19T12:10:00.000Z");
  });

  it("'+1 hour' emits now + 60 minutes (timezone-agnostic)", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={NOON_2026_04_19}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-quick-trigger"));
    await user.click(screen.getByTestId("schedule-picker-quick-hour-1"));
    expect(onChange).toHaveBeenCalledWith("2026-04-19T13:00:00.000Z");
  });

  it("'+24 hours' emits now + 24h (crosses midnight)", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={NOON_2026_04_19}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-quick-trigger"));
    await user.click(screen.getByTestId("schedule-picker-quick-hour-24"));
    expect(onChange).toHaveBeenCalledWith("2026-04-20T12:00:00.000Z");
  });

  it("'+3 days' emits now + 3 calendar days", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={NOON_2026_04_19}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-quick-trigger"));
    await user.click(screen.getByTestId("schedule-picker-quick-day-3"));
    expect(onChange).toHaveBeenCalledWith("2026-04-22T12:00:00.000Z");
  });

  it("'+2 weeks' emits now + 14 days", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={NOON_2026_04_19}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-quick-trigger"));
    await user.click(screen.getByTestId("schedule-picker-quick-week-2"));
    expect(onChange).toHaveBeenCalledWith("2026-05-03T12:00:00.000Z");
  });

  it("'+1 month' uses calendar arithmetic (preserves day-of-month)", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const apr15 = new Date("2026-04-15T09:00:00Z").getTime();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={apr15}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-quick-trigger"));
    await user.click(screen.getByTestId("schedule-picker-quick-month-1"));
    expect(onChange).toHaveBeenCalledWith("2026-05-15T09:00:00.000Z");
  });

  it("clamps month arithmetic to the last day of a shorter target month", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const jan31 = new Date("2026-01-31T09:00:00Z").getTime();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={jan31}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-quick-trigger"));
    await user.click(screen.getByTestId("schedule-picker-quick-month-1"));
    // Feb 31 doesn't exist; clamp to Feb 28 (2026 is not a leap year).
    expect(onChange).toHaveBeenCalledWith("2026-02-28T09:00:00.000Z");
  });

  it("closes the popover after a chip is picked", async () => {
    const user = userEvent.setup();
    render(
      <SchedulePicker
        value={null}
        onChange={() => {}}
        appTimezone={NY}
        nowMs={NOON_2026_04_19}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-quick-trigger"));
    expect(
      screen.getByTestId("schedule-picker-quick-popover"),
    ).toBeInTheDocument();
    await user.click(screen.getByTestId("schedule-picker-quick-hour-1"));
    expect(
      screen.queryByTestId("schedule-picker-quick-popover"),
    ).not.toBeInTheDocument();
  });

  it("Escape closes the popover without emitting a schedule", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={NOON_2026_04_19}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-quick-trigger"));
    await user.keyboard("{Escape}");
    expect(
      screen.queryByTestId("schedule-picker-quick-popover"),
    ).not.toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("aria-labels each chip with the screen-reader-friendly offset", async () => {
    const user = userEvent.setup();
    render(
      <SchedulePicker
        value={null}
        onChange={() => {}}
        appTimezone="UTC"
        nowMs={NOON_2026_04_19}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-quick-trigger"));
    expect(
      screen.getByRole("button", { name: /^schedule for 1 hour from now$/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /^schedule for 3 days from now$/i }),
    ).toBeInTheDocument();
  });
});
