import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { CustomSelect, type CustomSelectOption } from "./CustomSelect";

const OPTIONS: CustomSelectOption[] = [
  { value: "ready", label: "Ready" },
  { value: "running", label: "Running" },
];

describe("CustomSelect", () => {
  it("closes the listbox when tabbing away", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(
      <>
        <CustomSelect
          id="status"
          label="Status"
          value="ready"
          options={OPTIONS}
          onChange={onChange}
        />
        <button type="button">After field</button>
      </>,
    );

    await user.click(screen.getByRole("combobox", { name: /status/i }));
    expect(screen.getByRole("listbox", { name: /status/i })).toBeInTheDocument();

    await user.tab();
    await waitFor(() => {
      expect(
        screen.queryByRole("listbox", { name: /status/i }),
      ).not.toBeInTheDocument();
    });
  });
});
