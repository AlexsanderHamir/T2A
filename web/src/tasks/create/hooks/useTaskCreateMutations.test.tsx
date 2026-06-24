import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { taskQueryKeys } from "../../task-query";
import { useTaskCreateMutations } from "./useTaskCreateMutations";

vi.mock("@/api", () => ({
  createTask: vi.fn(),
  deleteTaskDraft: vi.fn(),
  deleteTaskTemplate: vi.fn(),
  getTaskDraft: vi.fn(),
  getTaskTemplate: vi.fn(),
  instantiateTaskTemplates: vi.fn(),
  patchTaskTemplate: vi.fn(),
  saveTaskDraft: vi.fn(),
  saveTaskTemplate: vi.fn(),
}));

import { instantiateTaskTemplates } from "@/api";

const mockedInstantiate = vi.mocked(instantiateTaskTemplates);

import { makeTask } from "@/test/taskDefaults";
function makeMutationInput(queryClient: QueryClient) {
  return {
    queryClient,
    newDraftIDRef: { current: "draft-1" },
    newDraftID: "draft-1",
    closeCreateModal: vi.fn(),
    setNewDraftID: vi.fn(),
    setDraftAutosaveBaseline: vi.fn(),
    setDraftAutosaveBaselineID: vi.fn(),
    setLastDraftSavedAt: vi.fn(),
    createModalOpen: false,
    editingTemplateId: null,
  };
}

describe("useTaskCreateMutations", () => {
  it("instantiate resolves before slow cache invalidation finishes", async () => {
    mockedInstantiate.mockResolvedValue({
      tasks: [makeTask()],
      errors: [],
    });

    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });

    let resolveInvalidate: () => void = () => {};
    const slowInvalidate = new Promise<void>((resolve) => {
      resolveInvalidate = resolve;
    });
    let invalidatePending = false;
    const invalidateSpy = vi
      .spyOn(queryClient, "invalidateQueries")
      .mockImplementation(() => {
        invalidatePending = true;
        return slowInvalidate;
      });

    function Wrapper({ children }: { children: ReactNode }) {
      return (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      );
    }

    const { result } = renderHook(
      () => useTaskCreateMutations(makeMutationInput(queryClient)),
      { wrapper: Wrapper },
    );

    let mutationSettled = false;
    await act(async () => {
      await result.current.instantiateTemplatesMutation
        .mutateAsync([{ template_id: "tmpl-1", count: 1 }])
        .then(() => {
          mutationSettled = true;
        });
    });

    expect(mutationSettled).toBe(true);
    expect(invalidatePending).toBe(true);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: taskQueryKeys.all });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: taskQueryKeys.stats() });

    await waitFor(() => {
      expect(result.current.instantiateTemplatesMutation.isSuccess).toBe(true);
    });

    resolveInvalidate();
  });
});
