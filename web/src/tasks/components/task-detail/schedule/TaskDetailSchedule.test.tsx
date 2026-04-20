import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AppSettings, ListCursorModelsResult } from "@/api/settings";
import { settingsQueryKeys } from "@/tasks/task-query/queryKeys";
import type { Status } from "@/types";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import { TaskDetailSchedule } from "./TaskDetailSchedule";

type FetchInput = Parameters<typeof fetch>[0];

const NY_SETTINGS: AppSettings = {
  worker_enabled: false,
  agent_paused: false,
  repo_root: "",
  cursor_bin: "",
  ...TASK_TEST_DEFAULTS,
  max_run_duration_seconds: 0,
  agent_pickup_delay_seconds: 0,
  display_timezone: "America/New_York",
};

const EMPTY_MODELS: ListCursorModelsResult = {
  ok: true,
  runner: TASK_TEST_DEFAULTS.runner,
  models: [],
};

function createWrapper(settings: AppSettings = NY_SETTINGS) {
  const qc = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, staleTime: Infinity },
    },
  });
  // Seed display_timezone synchronously so the picker renders in NY
  // without waiting for a fetch round-trip.
  qc.setQueryData(settingsQueryKeys.app(), settings);
  qc.setQueryData(
    [...settingsQueryKeys.all, "create-modal-cursor-models", "cursor", ""],
    EMPTY_MODELS,
  );
  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  }
  return { qc, Wrapper };
}

function renderPanel(opts: {
  status?: Status;
  pickup?: string | null;
  settings?: AppSettings;
} = {}) {
  const { Wrapper } = createWrapper(opts.settings);
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

const reqUrl = (input: FetchInput): string =>
  typeof input === "string"
    ? input
    : input instanceof URL
      ? input.toString()
      : (input as Request).url;

const reqMethod = (input: FetchInput, init?: RequestInit): string =>
  init?.method ?? (input instanceof Request ? input.method : "GET");

const reqBodyJson = async (
  input: FetchInput,
  init?: RequestInit,
): Promise<unknown> => {
  const body = init?.body ?? (input instanceof Request ? await input.text() : null);
  if (body == null) return null;
  if (typeof body === "string") return JSON.parse(body);
  return null;
};

const okJSON = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });

const errJSON = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });

beforeEach(() => {
  vi.spyOn(globalThis, "fetch").mockReset();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("TaskDetailSchedule visibility", () => {
  it("renders nothing when the task is terminal AND has no schedule", () => {
    renderPanel({ status: "done", pickup: null });
    expect(screen.queryByTestId("task-detail-schedule")).toBeNull();
  });

  it("renders nothing when the task is failed AND has no schedule", () => {
    renderPanel({ status: "failed", pickup: null });
    expect(screen.queryByTestId("task-detail-schedule")).toBeNull();
  });

  it("renders a read-only badge for a terminal task that still has a schedule", () => {
    renderPanel({
      status: "done",
      pickup: "2026-04-22T13:00:00Z",
    });
    expect(screen.getByTestId("task-detail-schedule")).toBeInTheDocument();
    expect(
      screen.getByTestId("task-detail-schedule-badge"),
    ).toBeInTheDocument();
    // No edit/clear buttons for terminal tasks.
    expect(
      screen.queryByTestId("task-detail-schedule-edit"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("task-detail-schedule-clear"),
    ).not.toBeInTheDocument();
  });

  it("renders the empty state + 'Schedule' button for an unscheduled non-terminal task", () => {
    renderPanel({ status: "ready", pickup: null });
    expect(screen.getByText(/no pickup scheduled/i)).toBeInTheDocument();
    expect(
      screen.getByTestId("task-detail-schedule-edit"),
    ).toHaveTextContent(/schedule/i);
    expect(
      screen.queryByTestId("task-detail-schedule-clear"),
    ).not.toBeInTheDocument();
  });

  it("renders badge formatted in the app timezone for a scheduled non-terminal task", () => {
    renderPanel({
      status: "ready",
      pickup: "2026-04-22T13:00:00Z",
    });
    // 13:00Z = 09:00 EDT in April.
    const badge = screen.getByTestId("task-detail-schedule-badge");
    expect(badge).toHaveTextContent(/scheduled for/i);
    expect(badge).toHaveTextContent(/09:00/);
    // Both controls present.
    expect(
      screen.getByTestId("task-detail-schedule-edit"),
    ).toHaveTextContent(/edit/i);
    expect(
      screen.getByTestId("task-detail-schedule-clear"),
    ).toBeInTheDocument();
  });
});

describe("TaskDetailSchedule clear", () => {
  it("PATCHes pickup_not_before: null when Clear is clicked", async () => {
    const user = userEvent.setup();
    const calls: { url: string; method: string; body: unknown }[] = [];
    vi.mocked(globalThis.fetch).mockImplementation(
      async (input: FetchInput, init?: RequestInit) => {
        const url = reqUrl(input);
        const method = reqMethod(input, init);
        const body = await reqBodyJson(input, init);
        calls.push({ url, method, body });
        if (method === "PATCH") {
          return okJSON({
            id: "task-1",
            title: "t",
            initial_prompt: "p",
            status: "ready",
            priority: "medium",
            task_type: "general",
            runner: "cursor",
            cursor_model: "",
            checklist_inherit: false,
            pickup_not_before: null,
          });
        }
        return okJSON({});
      },
    );

    renderPanel({
      status: "ready",
      pickup: "2026-04-22T13:00:00Z",
    });

    await user.click(screen.getByTestId("task-detail-schedule-clear"));
    await waitFor(() => {
      const patches = calls.filter((c) => c.method === "PATCH");
      expect(patches).toHaveLength(1);
    });
    const patch = calls.filter((c) => c.method === "PATCH")[0];
    expect(patch.url).toMatch(/\/tasks\/task-1$/);
    expect(patch.body).toEqual({ pickup_not_before: null });
  });

  it("surfaces an inline error when the Clear PATCH fails", async () => {
    const user = userEvent.setup();
    vi.mocked(globalThis.fetch).mockImplementation(async () =>
      errJSON(500, { error: "boom" }),
    );
    renderPanel({
      status: "ready",
      pickup: "2026-04-22T13:00:00Z",
    });
    await user.click(screen.getByTestId("task-detail-schedule-clear"));
    await waitFor(() => {
      expect(
        screen.getByTestId("task-detail-schedule-err"),
      ).toBeInTheDocument();
    });
  });
});

describe("TaskDetailSchedule edit modal", () => {
  it("opens the editor with the current schedule pre-populated", async () => {
    const user = userEvent.setup();
    renderPanel({
      status: "ready",
      pickup: "2026-04-22T13:00:00Z",
    });
    await user.click(screen.getByTestId("task-detail-schedule-edit"));
    expect(
      screen.getByRole("heading", { name: /edit pickup schedule/i }),
    ).toBeInTheDocument();
    const input = screen.getByTestId(
      "schedule-picker-input",
    ) as HTMLInputElement;
    // 13:00Z = 09:00 EDT.
    expect(input.value).toBe("2026-04-22T09:00");
  });

  it("PATCHes the chosen schedule when Save is clicked", async () => {
    const user = userEvent.setup();
    const calls: { url: string; method: string; body: unknown }[] = [];
    vi.mocked(globalThis.fetch).mockImplementation(
      async (input: FetchInput, init?: RequestInit) => {
        const url = reqUrl(input);
        const method = reqMethod(input, init);
        const body = await reqBodyJson(input, init);
        calls.push({ url, method, body });
        return okJSON({
          id: "task-1",
          title: "t",
          initial_prompt: "p",
          status: "ready",
          priority: "medium",
          task_type: "general",
          runner: "cursor",
          cursor_model: "",
          checklist_inherit: false,
          pickup_not_before: "2026-04-23T13:00:00Z",
        });
      },
    );
    renderPanel({ status: "ready", pickup: null });
    await user.click(screen.getByTestId("task-detail-schedule-edit"));
    // Use a quick-pick to avoid manually typing a datetime-local in
    // jsdom. "Tomorrow 9 AM" with no fixed clock just needs to emit
    // *some* future schedule string; we assert the body has the
    // pickup_not_before key set to a non-null RFC3339 string.
    await user.click(screen.getByTestId("schedule-picker-tomorrow"));
    await user.click(screen.getByRole("button", { name: /save schedule/i }));
    await waitFor(() => {
      const patches = calls.filter((c) => c.method === "PATCH");
      expect(patches).toHaveLength(1);
    });
    const patch = calls.filter((c) => c.method === "PATCH")[0];
    const body = patch.body as { pickup_not_before?: unknown };
    expect(typeof body.pickup_not_before).toBe("string");
    expect(body.pickup_not_before).toMatch(
      /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/,
    );
  });

  it("'Schedule' button on an unscheduled task opens the modal with no pre-populated value", async () => {
    const user = userEvent.setup();
    renderPanel({ status: "ready", pickup: null });
    await user.click(screen.getByTestId("task-detail-schedule-edit"));
    const input = screen.getByTestId(
      "schedule-picker-input",
    ) as HTMLInputElement;
    expect(input.value).toBe("");
  });
});
