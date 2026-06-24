import { screen, within, act, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  draftCreate,
  draftCreateCapture,
  draftsListEmpty,
} from "@/test/handlers/drafts";
import { listCursorModelsOk } from "@/test/handlers/settings";
import { openNewTaskModal } from "@/test/integration/createModalHelpers";
import {
  appDefaultHandlers,
  renderApp,
  setupAppTest,
} from "@/test/integration/appHarness";
import { server } from "@/test/server";

describe("draft autosave on create modal", () => {
  beforeEach(() => {
    setupAppTest();
    server.use(
      ...appDefaultHandlers(),
      listCursorModelsOk(),
      draftsListEmpty(),
    );
  });

  it("keeps draft autosave failures inside the modal", async () => {
    const user = userEvent.setup();
    server.use(draftCreate(404, { error: "Not Found" }));

    renderApp();
    await screen.findByText("No tasks yet");

    const dialog = await openNewTaskModal(user);
    await user.type(within(dialog).getByLabelText(/^title$/i), "Autosave test");

    expect(
      await within(dialog).findByText(
        /Draft autosave failed\. You can still create the task\./i,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("does not submit manual save when draft has no changes", async () => {
    const user = userEvent.setup();
    const draftSaves: string[] = [];
    server.use(
      draftCreateCapture(
        (body) => draftSaves.push(body),
        { status: 404, body: { error: "Not Found" } },
      ),
    );

    renderApp();
    await screen.findByText("No tasks yet");

    const dialog = await openNewTaskModal(user);
    await user.click(within(dialog).getByRole("button", { name: /^save draft$/i }));

    expect(draftSaves).toHaveLength(0);
    expect(
      within(dialog).queryByText(
        /Draft autosave failed\. You can still create the task\./i,
      ),
    ).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("clears prior autosave error when create modal is reopened", async () => {
    const user = userEvent.setup();
    server.use(draftCreate(404, { error: "Not Found" }));

    renderApp();
    await screen.findByText("No tasks yet");

    const firstDialog = await openNewTaskModal(user);
    await user.type(within(firstDialog).getByLabelText(/^title$/i), "trigger autosave");
    expect(
      await within(firstDialog).findByText(
        /Draft autosave failed\. You can still create the task\./i,
      ),
    ).toBeInTheDocument();

    await user.click(within(firstDialog).getByRole("button", { name: /^cancel$/i }));

    const secondDialog = await openNewTaskModal(user);
    expect(
      within(secondDialog).queryByText(
        /Draft autosave failed\. You can still create the task\./i,
      ),
    ).toBeNull();
  });

  it("does not autosave untouched fresh drafts", async () => {
    const draftSaves: string[] = [];
    server.use(
      draftCreateCapture(
        (body) => draftSaves.push(body),
        { status: 201, body: { id: "d1", name: "Untitled draft" } },
      ),
    );

    renderApp();
    await screen.findByText("No tasks yet");

    vi.useFakeTimers();
    try {
      fireEvent.click(screen.getByRole("button", { name: /^new task$/i }));
      expect(
        screen.getByRole("dialog", { name: /^new task$/i }),
      ).toBeInTheDocument();
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1200);
      });
      expect(draftSaves).toHaveLength(0);
    } finally {
      vi.useRealTimers();
    }
  });
});
