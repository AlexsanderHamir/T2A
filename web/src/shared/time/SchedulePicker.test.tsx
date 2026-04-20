import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SchedulePicker } from "./SchedulePicker";

const NY = "America/New_York";
const TOKYO = "Asia/Tokyo";

// 2026-04-19T13:00:00Z is a Sunday: 09:00 EDT (NY, UTC-4 in April
// after DST starts) and 22:00 JST (Tokyo, UTC+9, no DST). Anchoring
// on a Sunday means "Next Monday" is unambiguously +1 day and lets
// us assert wall-clock rendering for both eastward and westward
// timezones.
const SUNDAY_13Z = new Date("2026-04-19T13:00:00Z").getTime();

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
});

describe("SchedulePicker quick picks", () => {
  it("'In 1 hour' emits now + 60 minutes (timezone-agnostic)", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={SUNDAY_13Z}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-in-1h"));
    expect(onChange).toHaveBeenCalledWith("2026-04-19T14:00:00.000Z");
  });

  it("'Tonight 9 PM' (NY) emits today 21:00 EDT = 01:00 UTC next day", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={SUNDAY_13Z}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-tonight"));
    // 21:00 EDT on 2026-04-19 = 2026-04-20T01:00Z.
    expect(onChange).toHaveBeenCalledWith("2026-04-20T01:00:00.000Z");
  });

  it("'Tonight 9 PM' falls forward to tomorrow when 21:00 already passed", async () => {
    // 03:00Z next day = 23:00 EDT same day; 21:00 EDT has passed.
    const lateNight = new Date("2026-04-20T03:00:00Z").getTime();
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={lateNight}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-tonight"));
    // Should be 21:00 EDT on the *following* calendar day in NY
    // (2026-04-20 in NY = 23:00 of 04-19 + 4h = 04-20 in NY around
    // 23:00). Calling "tonight" at 23:00 EDT on 04-19 (= 03:00Z 04-20)
    // means 21:00 EDT on 04-20 = 01:00Z 04-21.
    expect(onChange).toHaveBeenCalledWith("2026-04-21T01:00:00.000Z");
  });

  it("'Tomorrow 9 AM' (NY) emits tomorrow 09:00 EDT = 13:00 UTC", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={SUNDAY_13Z}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-tomorrow"));
    expect(onChange).toHaveBeenCalledWith("2026-04-20T13:00:00.000Z");
  });

  it("'Next Monday 9 AM' from a Sunday is +1 day (NY)", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={SUNDAY_13Z}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-next-monday"));
    expect(onChange).toHaveBeenCalledWith("2026-04-20T13:00:00.000Z");
  });

  it("'Next Monday 9 AM' from a Monday is +7 days (never 'today' on Monday)", async () => {
    // 2026-04-20 is the Monday after our Sunday anchor. 13:00Z = 09:00 EDT.
    const monday = new Date("2026-04-20T13:00:00Z").getTime();
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={monday}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-next-monday"));
    // Next Monday is 2026-04-27 09:00 EDT = 13:00 UTC.
    expect(onChange).toHaveBeenCalledWith("2026-04-27T13:00:00.000Z");
  });

  it("'Clear' chip emits null", async () => {
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

  it("DST forward: a 9 AM local pick on the spring-forward day produces the correct UTC offset", async () => {
    // 2026-03-08 is the US spring-forward Sunday. At 02:00 EDT clocks
    // jump to 03:00 EDT. Clicking 'Tomorrow 9 AM' from 2026-03-07 (a
    // Saturday before the cliff) should land on 09:00 EDT on 03-08,
    // which is 13:00 UTC (UTC-4, post-jump), NOT 14:00 UTC (which
    // would be UTC-5 EST, the pre-jump offset). Catches the classic
    // "guessed offset at the source instant" bug.
    const beforeSpringForward = new Date("2026-03-07T15:00:00Z").getTime();
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={beforeSpringForward}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-tomorrow"));
    expect(onChange).toHaveBeenCalledWith("2026-03-08T13:00:00.000Z");
  });

  it("DST backward: a 9 AM local pick on the fall-back day produces the correct UTC offset", async () => {
    // 2026-11-01 is the US fall-back Sunday. Clicking 'Tomorrow 9 AM'
    // from 2026-10-31 (Saturday before the fall-back) should land on
    // 09:00 EST on 11-01, which is 14:00 UTC (UTC-5, post-fallback).
    const beforeFallBack = new Date("2026-10-31T15:00:00Z").getTime();
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={null}
        onChange={onChange}
        appTimezone={NY}
        nowMs={beforeFallBack}
      />,
    );
    await user.click(screen.getByTestId("schedule-picker-tomorrow"));
    expect(onChange).toHaveBeenCalledWith("2026-11-01T14:00:00.000Z");
  });
});
