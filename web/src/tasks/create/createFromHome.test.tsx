import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import { draftsListEmpty } from "@/test/handlers/drafts";
import { listCursorModelsOk } from "@/test/handlers/settings";
import {
  defaultTask,
  taskCreateFlowHandlers,
  tasksList,
} from "@/test/handlers/tasks";
import {
  addCriterionInDialog,
  choosePriorityInDialog,
  openNewTaskModal,
  waitForCreateTaskEnabled,
} from "@/test/integration/createModalHelpers";
import {
  appDefaultHandlers,
  renderApp,
  setupAppTest,
} from "@/test/integration/appHarness";
import { server } from "@/test/server";

describe("create task from home", () => {
  beforeEach(() => {
    setupAppTest();
    server.use(...appDefaultHandlers(), listCursorModelsOk(), draftsListEmpty());
  });

  it("creates a task and shows it in the table after refresh", async () => {
    const user = userEvent.setup();
    server.use(...taskCreateFlowHandlers({ taskId: "t1", title: "Ship fix" }));

    renderApp();
    await screen.findByText("No tasks yet");

    const dialog = await openNewTaskModal(user);
    await user.type(within(dialog).getByLabelText(/^title$/i), "Ship fix");
    await choosePriorityInDialog(user, dialog);
    await addCriterionInDialog(user, dialog, "Ship criterion");
    await waitForCreateTaskEnabled(dialog);
    await user.click(
      within(dialog).getByRole("button", { name: /^create task$/i }),
    );

    expect(
      await screen.findByRole("link", { name: /ship fix/i }),
    ).toBeInTheDocument();
  });

  it("creates a top-level task with checklist criteria in the create POST", async () => {
    const user = userEvent.setup();
    const createBodies: string[] = [];
    server.use(
      ...taskCreateFlowHandlers({
        taskId: "t1",
        title: "With criteria",
        onPost: (body) => createBodies.push(JSON.stringify(body)),
      }),
    );

    renderApp();
    await screen.findByText("No tasks yet");

    const dialog = await openNewTaskModal(user);
    await user.type(within(dialog).getByLabelText(/^title$/i), "With criteria");
    await choosePriorityInDialog(user, dialog);
    await user.click(
      within(dialog).getByRole("button", { name: /new criterion/i }),
    );
    const criterionDialog = await screen.findByRole("dialog", {
      name: /new criterion/i,
    });
    await user.type(
      within(criterionDialog).getByLabelText(/^criterion$/i),
      "Tests pass",
    );
    await user.click(
      within(criterionDialog).getByRole("button", { name: /^add criterion$/i }),
    );

    await waitForCreateTaskEnabled(dialog);
    await user.click(
      within(dialog).getByRole("button", { name: /^create task$/i }),
    );

    expect(
      await screen.findByRole("link", { name: /with criteria/i }),
    ).toBeInTheDocument();
    expect(createBodies).toHaveLength(1);
    expect(createBodies[0]).toContain("Tests pass");
    expect(createBodies[0]).toContain("checklist_items");
  });

  it("creates a top-level task using edited checklist criterion text", async () => {
    const user = userEvent.setup();
    const createBodies: string[] = [];
    server.use(
      ...taskCreateFlowHandlers({
        taskId: "t1",
        title: "With edited criteria",
        onPost: (body) => createBodies.push(JSON.stringify(body)),
      }),
    );

    renderApp();
    await screen.findByText("No tasks yet");

    const dialog = await openNewTaskModal(user);
    await user.type(
      within(dialog).getByLabelText(/^title$/i),
      "With edited criteria",
    );
    await choosePriorityInDialog(user, dialog);
    await user.click(
      within(dialog).getByRole("button", { name: /new criterion/i }),
    );
    const addCriterionDialog = await screen.findByRole("dialog", {
      name: /new criterion/i,
    });
    await user.type(
      within(addCriterionDialog).getByLabelText(/^criterion$/i),
      "Old wording",
    );
    await user.click(
      within(addCriterionDialog).getByRole("button", { name: /^add criterion$/i }),
    );

    await user.click(within(dialog).getByRole("button", { name: /^edit$/i }));
    const editCriterionDialog = await screen.findByRole("dialog", {
      name: /edit criterion/i,
    });
    const criterionInput = within(editCriterionDialog).getByLabelText(
      /^criterion$/i,
    );
    await user.clear(criterionInput);
    await user.type(criterionInput, "Updated wording");
    await user.click(
      within(editCriterionDialog).getByRole("button", { name: /^save changes$/i }),
    );

    await waitForCreateTaskEnabled(dialog);
    await user.click(
      within(dialog).getByRole("button", { name: /^create task$/i }),
    );

    expect(
      await screen.findByRole("link", { name: /with edited criteria/i }),
    ).toBeInTheDocument();
    expect(createBodies).toHaveLength(1);
    expect(createBodies[0]).toContain("Updated wording");
    expect(createBodies[0]).toContain("checklist_items");
  });

  it("does not expose a parent picker on the home create-task modal", async () => {
    const user = userEvent.setup();
    server.use(
      tasksList([
        defaultTask("p1", "Check parent"),
      ]),
    );

    renderApp();
    expect(await screen.findByText("Check parent")).toBeInTheDocument();

    const dialog = await openNewTaskModal(user);
    expect(
      within(dialog).getByRole("heading", { name: /^new task$/i }),
    ).toBeInTheDocument();
    expect(
      within(dialog).queryByRole("combobox", { name: /^parent task$/i }),
    ).not.toBeInTheDocument();
    expect(
      within(dialog).queryByText(/inherit parent's checklist criteria/i),
    ).not.toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: /^create task$/i }),
    ).toBeInTheDocument();
    expect(
      within(dialog).queryByRole("button", { name: /^add subtask$/i }),
    ).not.toBeInTheDocument();
  });

  it("posts a top-level task with no parent_id from the home create modal", async () => {
    const user = userEvent.setup();
    let postBody: Record<string, unknown> | null = null;
    server.use(
      ...taskCreateFlowHandlers({
        taskId: "new",
        title: "Standalone task",
        seedTasks: [defaultTask("parent", "Parent task")],
        onPost: (body) => {
          postBody = body as Record<string, unknown>;
        },
      }),
    );

    renderApp();
    expect(await screen.findByText("Parent task")).toBeInTheDocument();

    const dialog = await openNewTaskModal(user);
    await user.type(
      within(dialog).getByLabelText(/^title$/i),
      "Standalone task",
    );
    await choosePriorityInDialog(user, dialog);
    await addCriterionInDialog(user, dialog, "Standalone criterion");
    await waitForCreateTaskEnabled(dialog);
    await user.click(
      within(dialog).getByRole("button", { name: /^create task$/i }),
    );

    await waitFor(() => {
      expect(postBody).not.toBeNull();
    });
    const posted = postBody as unknown as {
      parent_id?: unknown;
      title?: unknown;
    };
    expect(posted.title).toBe("Standalone task");
    expect(posted.parent_id).toBeUndefined();
  });
});
