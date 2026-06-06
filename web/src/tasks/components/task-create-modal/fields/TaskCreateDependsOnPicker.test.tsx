import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { ModalStackProvider } from "@/shared/ModalStackContext";
import { taskQueryKeys } from "@/tasks/task-query";
import type { Task, TaskListResponse } from "@/types";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import { TaskCreateDependsOnPicker } from "./TaskCreateDependsOnPicker";

function makeTask(partial: Partial<Task> & Pick<Task, "id" | "title">): Task {
  return {
    initial_prompt: "",
    status: "ready",
    priority: "medium",
    runner: TASK_TEST_DEFAULTS.runner,
    cursor_model: TASK_TEST_DEFAULTS.cursor_model,
    checklist_inherit: false,
    ...partial,
  } as Task;
}

const PROJECT_A = "project-aaa";
const PROJECT_B = "project-bbb";

const PROJECT_A_TASKS: Task[] = [
  makeTask({
    id: "11111111-1111-4111-8111-111111111111",
    title: "Authentication API",
    project_id: PROJECT_A,
  }),
  makeTask({
    id: "22222222-2222-4222-8222-222222222222",
    title: "Login form",
    project_id: PROJECT_A,
  }),
  makeTask({
    id: "33333333-3333-4333-8333-333333333333",
    title: "Password reset",
    project_id: PROJECT_A,
  }),
];

const PROJECT_B_TASK = makeTask({
  id: "99999999-9999-4999-8999-999999999999",
  title: "Out of scope task",
  project_id: PROJECT_B,
});

function renderPicker(props?: {
  projectId?: string;
  selected?: string[];
  disabled?: boolean;
}) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity },
      mutations: { retry: false },
    },
  });
  const list: TaskListResponse = {
    tasks: [...PROJECT_A_TASKS, PROJECT_B_TASK],
    limit: 200,
    offset: 0,
    has_more: false,
  };
  queryClient.setQueryData(
    taskQueryKeys.list({ limit: 200, offset: 0 }),
    list,
  );

  const onChange = vi.fn();

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <ModalStackProvider>{children}</ModalStackProvider>
      </QueryClientProvider>
    );
  }

  const utils = render(
    <TaskCreateDependsOnPicker
      projectId={props?.projectId ?? PROJECT_A}
      selected={props?.selected ?? []}
      onChange={onChange}
      disabled={props?.disabled ?? false}
    />,
    { wrapper: Wrapper },
  );

  return { onChange, ...utils };
}

describe("TaskCreateDependsOnPicker", () => {
  it("renders a disabled lookup with a nudge when no project is chosen", () => {
    renderPicker({ projectId: "" });
    expect(screen.getByRole("combobox")).toBeDisabled();
    expect(screen.getByRole("button", { name: /browse/i })).toBeDisabled();
    expect(
      screen.getByText(/pick a project first to add dependencies/i),
    ).toBeInTheDocument();
  });

  it("filters typeahead results to tasks from the active project", async () => {
    const user = userEvent.setup();
    renderPicker({ projectId: PROJECT_A });
    const input = screen.getByRole("combobox");
    await user.click(input);
    // The dropdown should show every Project A task and never the
    // Project B task — that's the whole point of project scoping.
    const listbox = await screen.findByRole("listbox");
    expect(within(listbox).getByText(/authentication api/i)).toBeInTheDocument();
    expect(within(listbox).getByText(/login form/i)).toBeInTheDocument();
    expect(within(listbox).getByText(/password reset/i)).toBeInTheDocument();
    expect(
      within(listbox).queryByText(/out of scope task/i),
    ).not.toBeInTheDocument();
  });

  it("narrows results by typing in the search input", async () => {
    const user = userEvent.setup();
    renderPicker({ projectId: PROJECT_A });
    const input = screen.getByRole("combobox");
    await user.click(input);
    await user.type(input, "login");
    const listbox = await screen.findByRole("listbox");
    expect(within(listbox).getByText(/login form/i)).toBeInTheDocument();
    expect(within(listbox).queryByText(/password reset/i)).not.toBeInTheDocument();
    expect(
      within(listbox).queryByText(/authentication api/i),
    ).not.toBeInTheDocument();
  });

  it("appends the picked task id to selected on click", async () => {
    const user = userEvent.setup();
    const { onChange } = renderPicker({
      projectId: PROJECT_A,
      selected: [PROJECT_A_TASKS[0].id],
    });
    const input = screen.getByRole("combobox");
    await user.click(input);
    const listbox = await screen.findByRole("listbox");
    // The already-selected task is excluded from the typeahead results so
    // operators can't accidentally add the same dependency twice.
    expect(
      within(listbox).queryByText(/authentication api/i),
    ).not.toBeInTheDocument();
    await user.click(within(listbox).getByText(/login form/i));
    expect(onChange).toHaveBeenCalledWith([
      PROJECT_A_TASKS[0].id,
      PROJECT_A_TASKS[1].id,
    ]);
  });

  it("removes a chip on click", async () => {
    const user = userEvent.setup();
    const { onChange } = renderPicker({
      projectId: PROJECT_A,
      selected: [PROJECT_A_TASKS[0].id, PROJECT_A_TASKS[1].id],
    });
    const chip = screen.getByRole("button", {
      name: /remove dependency authentication api/i,
    });
    await user.click(chip);
    expect(onChange).toHaveBeenCalledWith([PROJECT_A_TASKS[1].id]);
  });

  it("opens the browse modal listing every project task with a checkbox row", async () => {
    const user = userEvent.setup();
    renderPicker({ projectId: PROJECT_A });
    await user.click(screen.getByRole("button", { name: /browse/i }));
    expect(
      screen.getByRole("heading", { name: /project tasks/i }),
    ).toBeInTheDocument();
    // All three Project A tasks appear; Project B does not.
    expect(screen.getByText(/authentication api/i)).toBeInTheDocument();
    expect(screen.getByText(/login form/i)).toBeInTheDocument();
    expect(screen.getByText(/password reset/i)).toBeInTheDocument();
    expect(screen.queryByText(/out of scope task/i)).not.toBeInTheDocument();
  });

  it("toggles selection from the browse modal checkboxes", async () => {
    const user = userEvent.setup();
    const { onChange } = renderPicker({
      projectId: PROJECT_A,
      selected: [PROJECT_A_TASKS[0].id],
    });
    await user.click(screen.getByRole("button", { name: /browse/i }));
    const checkboxes = screen.getAllByRole("checkbox");
    // First checkbox corresponds to the first Project A task — already
    // selected, so toggling should remove it.
    expect(checkboxes[0]).toBeChecked();
    await user.click(checkboxes[0]);
    expect(onChange).toHaveBeenCalledWith([]);
  });
});
