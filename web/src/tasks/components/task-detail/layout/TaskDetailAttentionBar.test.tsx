import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { TaskDetailToolbarActions } from "./TaskDetailAttentionBar";

describe("TaskDetailToolbarActions", () => {
  it("invokes edit and delete handlers", async () => {
    const user = userEvent.setup();
    const onEdit = vi.fn();
    const onDelete = vi.fn();
    render(
      <TaskDetailToolbarActions
        saving={false}
        onEdit={onEdit}
        onDelete={onDelete}
      />,
    );

    await user.click(screen.getByRole("button", { name: /edit task/i }));
    await user.click(screen.getByRole("button", { name: /^delete$/i }));
    expect(onEdit).toHaveBeenCalledOnce();
    expect(onDelete).toHaveBeenCalledOnce();
  });

  it("disables action buttons while saving", () => {
    render(
      <TaskDetailToolbarActions
        saving
        onEdit={vi.fn()}
        onDelete={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: /edit task/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /^delete$/i })).toBeDisabled();
  });

  it("renders Start over and Resume from failure when retry handlers are provided", async () => {
    const user = userEvent.setup();
    const onRetryFresh = vi.fn();
    const onRetryResume = vi.fn();
    render(
      <TaskDetailToolbarActions
        saving={false}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onRetryFresh={onRetryFresh}
        onRetryResume={onRetryResume}
      />,
    );

    await user.click(screen.getByRole("button", { name: /^start over$/i }));
    await user.click(
      screen.getByRole("button", { name: /^resume from failure$/i }),
    );
    expect(onRetryFresh).toHaveBeenCalledOnce();
    expect(onRetryResume).toHaveBeenCalledOnce();
  });

  it("renders Model configuration only when showModelConfig is true", async () => {
    const user = userEvent.setup();
    const onConfigureModel = vi.fn();

    const { rerender } = render(
      <TaskDetailToolbarActions
        saving={false}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onConfigureModel={onConfigureModel}
      />,
    );
    expect(
      screen.queryByRole("button", { name: /model configuration/i }),
    ).not.toBeInTheDocument();

    rerender(
      <TaskDetailToolbarActions
        saving={false}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onConfigureModel={onConfigureModel}
        showModelConfig
      />,
    );

    const button = screen.getByRole("button", {
      name: /model configuration/i,
    });
    await user.click(button);
    expect(onConfigureModel).toHaveBeenCalledOnce();
  });

  describe("autonomy toggle", () => {
    it("does not render the toggle when autonomyMode is hidden / unset", () => {
      render(
        <TaskDetailToolbarActions
          saving={false}
          onEdit={vi.fn()}
          onDelete={vi.fn()}
        />,
      );
      expect(
        screen.queryByRole("button", { name: /^(resume|put on hold)$/i }),
      ).not.toBeInTheDocument();
    });

    it("renders 'Put on hold' when autonomyMode=ready", async () => {
      const user = userEvent.setup();
      const onToggleAutonomy = vi.fn();
      render(
        <TaskDetailToolbarActions
          saving={false}
          onEdit={vi.fn()}
          onDelete={vi.fn()}
          autonomyMode="ready"
          onToggleAutonomy={onToggleAutonomy}
        />,
      );
      const button = screen.getByRole("button", { name: /^put on hold$/i });
      await user.click(button);
      expect(onToggleAutonomy).toHaveBeenCalledOnce();
    });

    it("renders 'Resume' when autonomyMode=on_hold", async () => {
      const user = userEvent.setup();
      const onToggleAutonomy = vi.fn();
      render(
        <TaskDetailToolbarActions
          saving={false}
          onEdit={vi.fn()}
          onDelete={vi.fn()}
          autonomyMode="on_hold"
          onToggleAutonomy={onToggleAutonomy}
        />,
      );
      const button = screen.getByRole("button", { name: /^resume$/i });
      await user.click(button);
      expect(onToggleAutonomy).toHaveBeenCalledOnce();
    });

    it("shows the pending label and disables the toggle while pending", () => {
      render(
        <TaskDetailToolbarActions
          saving={false}
          onEdit={vi.fn()}
          onDelete={vi.fn()}
          autonomyMode="ready"
          onToggleAutonomy={vi.fn()}
          autonomyPending
        />,
      );
      const button = screen.getByRole("button", { name: /^holding…$/i });
      expect(button).toBeDisabled();
    });
  });

  it("no longer renders the legacy inline model-config panel", () => {
    render(
      <TaskDetailToolbarActions
        saving={false}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onConfigureModel={vi.fn()}
        showModelConfig
      />,
    );

    expect(
      screen.queryByRole("heading", { name: /model configuration/i, level: 3 }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/global model/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/per-task model/i)).not.toBeInTheDocument();
  });
});
