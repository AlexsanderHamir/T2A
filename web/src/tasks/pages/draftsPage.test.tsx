import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { beforeEach, describe, expect, it } from "vitest";
import {
  draftDelete,
  draftDeletePending,
  draftsList,
  draftsListPending,
} from "@/test/handlers/drafts";
import {
  appDefaultHandlers,
  renderAppAt,
  setupAppTest,
} from "@/test/integration/appHarness";
import { server } from "@/test/server";

describe("task drafts page", () => {
  beforeEach(() => {
    setupAppTest();
    server.use(...appDefaultHandlers());
  });

  it("shows loading status on drafts page while drafts are fetching", async () => {
    const [pendingHandler, deferred] = draftsListPending();
    server.use(pendingHandler);

    renderAppAt(["/drafts"]);

    expect(await screen.findByRole("heading", { name: /^task drafts$/i })).toBeInTheDocument();
    expect(
      await screen.findByRole("status", { name: /loading drafts/i }),
    ).toBeInTheDocument();

    deferred.resolve(HttpResponse.json({ drafts: [] }));
  });

  it("shows an error on drafts page when draft list request fails", async () => {
    server.use(
      http.get("/task-drafts", () =>
        HttpResponse.json({ error: "drafts unavailable" }, { status: 500 }),
      ),
    );

    renderAppAt(["/drafts"]);

    expect(await screen.findByRole("alert")).toHaveTextContent(/drafts unavailable/i);
    expect(
      screen.getByRole("button", { name: /^try again$/i }),
    ).toBeInTheDocument();
  });

  it("retries draft list from drafts page after an error", async () => {
    const user = userEvent.setup();
    let draftAttempts = 0;
    server.use(
      http.get("/task-drafts", () => {
        draftAttempts += 1;
        if (draftAttempts === 1) {
          return HttpResponse.json({ error: "drafts unavailable" }, { status: 500 });
        }
        return HttpResponse.json({
          drafts: [
            {
              id: "d1",
              name: "Recovered",
              created_at: "2026-04-07T10:00:00Z",
              updated_at: "2026-04-07T10:05:00Z",
            },
          ],
        });
      }),
    );

    renderAppAt(["/drafts"]);

    expect(await screen.findByRole("alert")).toHaveTextContent(/drafts unavailable/i);
    await user.click(screen.getByRole("button", { name: /^try again$/i }));
    expect(
      await screen.findByRole("listitem", {
        name: /^resume draft: recovered$/i,
      }),
    ).toBeInTheDocument();
  });

  it("offers create task from drafts page when there are no drafts", async () => {
    const user = userEvent.setup();
    server.use(draftsList([]));

    renderAppAt(["/drafts"]);

    await screen.findByRole("heading", { name: /^task drafts$/i });
    await user.click(
      await screen.findByRole("button", { name: /^create a task$/i }),
    );
    expect(
      await screen.findByRole("dialog", { name: /^new task$/i }),
    ).toBeInTheDocument();
  });

  it("shows resume error on drafts page when opening a draft fails", async () => {
    const user = userEvent.setup();
    server.use(
      draftsList([
        {
          id: "d1",
          name: "Broken draft",
          created_at: "2026-04-07T10:00:00Z",
          updated_at: "2026-04-07T10:05:00Z",
        },
      ]),
      http.get("/task-drafts/d1", () =>
        HttpResponse.json({ error: "resume failed" }, { status: 500 }),
      ),
    );

    renderAppAt(["/drafts"]);

    await screen.findByRole("heading", { name: /^task drafts$/i });
    await user.click(
      await screen.findByRole("listitem", {
        name: /^resume draft: broken draft$/i,
      }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(/resume failed/i);
  });

  it("shows delete error on drafts page when deleting a draft fails", async () => {
    const user = userEvent.setup();
    server.use(
      draftsList([
        {
          id: "d1",
          name: "Delete me",
          created_at: "2026-04-07T10:00:00Z",
          updated_at: "2026-04-07T10:05:00Z",
        },
      ]),
      draftDelete("d1", 500, { error: "delete failed" }),
    );

    renderAppAt(["/drafts"]);

    await screen.findByRole("heading", { name: /^task drafts$/i });
    await user.click(
      await screen.findByRole("button", {
        name: /^delete draft "delete me"$/i,
      }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(/delete failed/i);
  });

  it("shows delete loading state only for clicked draft row", async () => {
    const user = userEvent.setup();
    const [deleteHandler, deleteDeferred] = draftDeletePending("d1");
    server.use(
      draftsList([
        {
          id: "d1",
          name: "First draft",
          created_at: "2026-04-07T10:00:00Z",
          updated_at: "2026-04-07T10:05:00Z",
        },
        {
          id: "d2",
          name: "Second draft",
          created_at: "2026-04-07T11:00:00Z",
          updated_at: "2026-04-07T11:05:00Z",
        },
      ]),
      deleteHandler,
    );

    renderAppAt(["/drafts"]);

    await screen.findByRole("heading", { name: /^task drafts$/i });
    const firstRow = await screen.findByRole("listitem", {
      name: /^resume draft: first draft$/i,
    });
    const secondRow = await screen.findByRole("listitem", {
      name: /^resume draft: second draft$/i,
    });

    await user.click(
      within(firstRow).getByRole("button", {
        name: /^delete draft "first draft"$/i,
      }),
    );

    expect(
      within(firstRow).getByRole("button", {
        name: /^deleting draft "first draft"$/i,
      }),
    ).toBeDisabled();
    expect(
      within(secondRow).getByRole("button", {
        name: /^delete draft "second draft"$/i,
      }),
    ).toBeInTheDocument();

    deleteDeferred.resolve(new HttpResponse(null, { status: 204 }));
  });
});
