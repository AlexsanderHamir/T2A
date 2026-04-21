import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import type { Task } from "@/types";
import { TaskChangeModelModal } from "./TaskChangeModelModal";

const TASK: Task = {
  id: "t1",
  title: "Example",
  initial_prompt: "",
  status: "failed",
  priority: "high",
  checklist_inherit: false,
  ...TASK_TEST_DEFAULTS,
};

function renderModal() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  vi.spyOn(globalThis, "fetch").mockResolvedValue(
    new Response(JSON.stringify({}), { status: 404 }),
  );
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <TaskChangeModelModal
          task={TASK}
          cursorModel=""
          onCursorModelChange={vi.fn()}
          saving={false}
          patchPending={false}
          onSubmit={vi.fn()}
          onCancel={vi.fn()}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("TaskChangeModelModal", () => {
  it("renders title and agent section", () => {
    renderModal();
    expect(
      screen.getByRole("heading", { name: /change model/i }),
    ).toBeInTheDocument();
    expect(screen.getByText(/example/i)).toBeInTheDocument();
  });
});
