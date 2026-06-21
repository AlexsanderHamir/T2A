import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { TimezoneCombobox } from "./TimezoneCombobox";

const OPTIONS = [
  { value: "UTC", label: "UTC — Coordinated Universal Time (GMT+00:00)" },
  {
    value: "Europe/Berlin",
    label: "Europe/Berlin — Central European Time (GMT+01:00)",
  },
];

describe("TimezoneCombobox", () => {
  it("opens the list and commits a timezone selection", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(
      <TimezoneCombobox
        value=""
        onChange={onChange}
        browserTz="America/Los_Angeles"
        options={OPTIONS}
      />,
    );

    await user.click(screen.getByTestId("settings-display-timezone-select"));
    const listbox = await screen.findByRole("listbox");
    await user.click(
      screen.getByRole("option", { name: /UTC — Coordinated Universal Time/i }),
    );

    await waitFor(() => {
      expect(listbox).not.toBeInTheDocument();
    });
    expect(onChange).toHaveBeenCalledWith("UTC");
  });
});
