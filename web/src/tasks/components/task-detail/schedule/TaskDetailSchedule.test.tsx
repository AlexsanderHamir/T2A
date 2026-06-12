import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";
import type { AppSettings } from "@/api/settings";
import { settingsQueryKeys } from "@/tasks/task-query/queryKeys";
import type { Status } from "@/types";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import { APP_SETTINGS_DEFAULTS } from "@/test/settingsDefaults";
import { TaskDetailSchedule } from "./TaskDetailSchedule";

const NY_SETTINGS: AppSettings = {
  ...APP_SETTINGS_DEFAULTS,
  ...TASK_TEST_DEFAULTS,
  display_timezone: "America/New_York",
};

function createWrapper(settings: AppSettings = NY_SETTINGS) {
  const qc = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, staleTime: Infinity },
    },
  });
  qc.setQueryData(settingsQueryKeys.app(), settings);
  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  }
  return { Wrapper };
}

function renderPanel(opts: {
  status?: Status;
  pickup?: string | null;
} = {}) {
  const { Wrapper } = createWrapper();
  const task = {
    id: "task-1",
    status: opts.status ?? ("ready" as Status),
    pickup_not_before: opts.pickup ?? undefined,
  };
  return render(
    <Wrapper>
      <TaskDetailSchedule task={task} />
    </Wrapper>,
  );
}

describe("TaskDetailSchedule (read-only)", () => {
  it("renders nothing when the task is terminal and has no schedule", () => {
    renderPanel({ status: "done", pickup: null });
    expect(screen.queryByTestId("task-detail-schedule")).toBeNull();
  });

  it("renders a badge for a scheduled task formatted in the app timezone", () => {
    renderPanel({
      status: "ready",
      pickup: "2026-04-22T13:00:00Z",
    });
    const badge = screen.getByTestId("task-detail-schedule-badge");
    expect(badge).toHaveTextContent(/scheduled for/i);
    expect(badge).toHaveTextContent(/09:00/);
  });

  it("shows empty copy for an unscheduled non-terminal task without action buttons", () => {
    renderPanel({ status: "ready", pickup: null });
    expect(screen.getByText(/no pickup scheduled/i)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /schedule/i }),
    ).not.toBeInTheDocument();
  });

  it("shows read-only schedule for a running task without edit controls", () => {
    renderPanel({
      status: "running",
      pickup: "2026-04-22T13:00:00Z",
    });
    expect(screen.getByTestId("task-detail-schedule-badge")).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
