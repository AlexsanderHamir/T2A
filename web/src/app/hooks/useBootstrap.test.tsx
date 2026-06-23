import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { settingsQueryKeys, taskQueryKeys } from "@/tasks/task-query";
import { projectQueryKeys } from "@/projects/queryKeys";
import { useBootstrap } from "./useBootstrap";

vi.mock("@/api", async () => {
  const actual = await vi.importActual<typeof import("@/api")>("@/api");
  return {
    ...actual,
    fetchBootstrap: vi.fn(),
  };
});

import { fetchBootstrap } from "@/api";

const mockedFetchBootstrap = vi.mocked(fetchBootstrap);

function makeQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

function makeWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

const SAMPLE_BOOTSTRAP = {
  settings: {
    agent_paused: false,
    runner: "cursor",
    cursor_bin: "",
    cursor_model: "",
    max_run_duration_seconds: 0,
    stream_idle_stuck_seconds: 60,
    agent_pickup_delay_seconds: 5,
    display_timezone: "UTC",
    optimistic_mutations_enabled: true,
    sse_replay_enabled: true,
    verify_max_retries: 1,
    verify_runner_name: "",
    verify_runner_model: "",
  },
  tasks: {
    tasks: [],
    limit: 20,
    offset: 0,
    has_more: false,
  },
  stats: {
    total: 0,
    by_status: {},
    by_priority: {},
    by_type: {},
    by_project: [],
    by_runner: { runners: [] },
    cycles: { running: 0, succeeded: 0, failed: 0 },
    phases: { running: 0, succeeded: 0, failed: 0 },
    recent_failures: [],
    overdue: 0,
    blocked: 0,
    gates_unmet: 0,
    ready: 0,
    in_progress: 0,
  },
  projects: { projects: [], limit: 100 },
  drafts: [],
};

describe("useBootstrap", () => {
  beforeEach(() => {
    mockedFetchBootstrap.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("seeds the cache for each consumer query key on success", async () => {
    mockedFetchBootstrap.mockResolvedValue(
      // Mocked payload bypasses runtime parsing; we cast through unknown
      // because the real Bootstrap type includes deeper task/stats shapes
      // that are exercised by parseTaskApi.test.ts already.
      SAMPLE_BOOTSTRAP as unknown as Awaited<ReturnType<typeof fetchBootstrap>>,
    );
    const queryClient = makeQueryClient();
    renderHook(() => useBootstrap(), {
      wrapper: makeWrapper(queryClient),
    });

    await waitFor(() => {
      expect(mockedFetchBootstrap).toHaveBeenCalledTimes(1);
    });
    await waitFor(() => {
      expect(queryClient.getQueryData(settingsQueryKeys.app())).toEqual(
        SAMPLE_BOOTSTRAP.settings,
      );
    });
    expect(
      queryClient.getQueryData(
        taskQueryKeys.list({ limit: 20, offset: 0 }),
      ),
    ).toEqual(SAMPLE_BOOTSTRAP.tasks);
    expect(queryClient.getQueryData(taskQueryKeys.stats())).toEqual(
      SAMPLE_BOOTSTRAP.stats,
    );
    expect(
      queryClient.getQueryData(projectQueryKeys.list(false, 100)),
    ).toEqual(SAMPLE_BOOTSTRAP.projects);
    expect(queryClient.getQueryData(taskQueryKeys.drafts())).toEqual(
      SAMPLE_BOOTSTRAP.drafts,
    );
  });

  it("silently does nothing when the endpoint is unavailable (null)", async () => {
    mockedFetchBootstrap.mockResolvedValue(null);
    const queryClient = makeQueryClient();
    renderHook(() => useBootstrap(), {
      wrapper: makeWrapper(queryClient),
    });

    await waitFor(() => {
      expect(mockedFetchBootstrap).toHaveBeenCalledTimes(1);
    });
    expect(queryClient.getQueryData(settingsQueryKeys.app())).toBeUndefined();
    expect(
      queryClient.getQueryData(taskQueryKeys.list({ limit: 20, offset: 0 })),
    ).toBeUndefined();
  });

  it("does not seed anything when the request fails", async () => {
    mockedFetchBootstrap.mockRejectedValue(new Error("network down"));
    const queryClient = makeQueryClient();
    renderHook(() => useBootstrap(), {
      wrapper: makeWrapper(queryClient),
    });

    await waitFor(() => {
      expect(mockedFetchBootstrap).toHaveBeenCalledTimes(1);
    });
    expect(queryClient.getQueryData(settingsQueryKeys.app())).toBeUndefined();
  });
});
