import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { FormEvent, ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Task } from "@/types";
import { DEFAULT_PROJECT_ID } from "@/types";
import { settingsQueryKeys } from "../task-query";
import { useTaskCreateFlow } from "./useTaskCreateFlow";

vi.mock("../../api", () => ({
  addChecklistItem: vi.fn(),
  createTask: vi.fn(),
  deleteTaskDraft: vi.fn(),
  evaluateDraftTask: vi.fn(),
  getTaskDraft: vi.fn(),
  listTaskDrafts: vi.fn(),
  saveTaskDraft: vi.fn(),
}));

import { createTask, listTaskDrafts } from "../../api";

const mockedCreateTask = vi.mocked(createTask);
const mockedListDrafts = vi.mocked(listTaskDrafts);

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    id: "task-1",
    title: "Fresh task",
    initial_prompt: "",
    status: "ready",
    priority: "medium",
    task_type: "general",
    runner: "cursor",
    cursor_model: "",
    checklist_inherit: false,
    project_id: DEFAULT_PROJECT_ID,
    ...overrides,
  };
}

function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  queryClient.setQueryData(settingsQueryKeys.app(), {
    worker_enabled: false,
    agent_paused: false,
    runner: "cursor",
    repo_root: "",
    cursor_bin: "",
    cursor_model: "",
    max_run_duration_seconds: 0,
    agent_pickup_delay_seconds: 5,
    display_timezone: "UTC",
    optimistic_mutations_enabled: false,
    sse_replay_enabled: false,
  });

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }

  return { Wrapper };
}

describe("useTaskCreateFlow", () => {
  beforeEach(() => {
    mockedCreateTask.mockResolvedValue(makeTask());
    mockedListDrafts.mockResolvedValue([]);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("submits the default project for a fresh task when the dropdown is left unchanged", async () => {
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTaskCreateFlow(), { wrapper: Wrapper });

    await waitFor(() => {
      expect(result.current.draftListLoading).toBe(false);
    });

    act(() => {
      result.current.openCreateModal();
      result.current.setNewTitle("Fresh task");
      result.current.setNewPriority("medium");
    });

    act(() => {
      result.current.submitCreate({
        preventDefault: vi.fn(),
      } as unknown as FormEvent);
    });

    await waitFor(() => {
      expect(mockedCreateTask).toHaveBeenCalledTimes(1);
    });
    expect(mockedCreateTask).toHaveBeenCalledWith(
      expect.objectContaining({ project_id: DEFAULT_PROJECT_ID }),
    );
  });
});
