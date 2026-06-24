import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it } from "vitest";
import {
  draftCreateCapture,
  draftGet,
  draftsList,
  draftsListPending,
} from "@/test/handlers/drafts";
import { listCursorModelsOk } from "@/test/handlers/settings";
import {
  appDefaultHandlers,
  renderApp,
  renderAppAt,
  setupAppTest,
} from "@/test/integration/appHarness";
import { server } from "@/test/server";

describe("draft entry hints", () => {
  beforeEach(() => {
    setupAppTest();
    server.use(...appDefaultHandlers());
  });

  it("opens a draft from drafts page in a prefilled create modal", async () => {
    const user = userEvent.setup();
    const draftSaves: string[] = [];
    server.use(
      draftsList([
        {
          id: "d1",
          name: "Draft from list",
          created_at: "2026-04-07T10:00:00Z",
          updated_at: "2026-04-07T10:05:00Z",
        },
      ]),
      draftGet("d1", {
        name: "Draft from list",
        created_at: "2026-04-07T10:00:00Z",
        updated_at: "2026-04-07T10:05:00Z",
        payload: {
          title: "Prefilled title",
          initial_prompt: "Prefilled prompt",
          priority: "high",
          checklist_items: ["Do step A"],
        },
      }),
      draftCreateCapture(
        (body) => draftSaves.push(body),
        { status: 200, body: { id: "d1", name: "Draft from list" } },
      ),
    );

    renderAppAt(["/drafts"]);

    await screen.findByRole("heading", { name: /^task drafts$/i });
    await user.click(
      await screen.findByRole("listitem", {
        name: /^resume draft: draft from list$/i,
      }),
    );

    const dialog = await screen.findByRole("dialog", { name: /^new task$/i });
    expect(within(dialog).queryByLabelText(/^draft name$/i)).not.toBeInTheDocument();
    expect(within(dialog).getByLabelText(/^title$/i)).toHaveValue("Prefilled title");
    expect(within(dialog).getByText("Do step A")).toBeInTheDocument();
    expect(draftSaves).toHaveLength(0);
  });

  it("shows loading status in draft picker modal from home", async () => {
    const user = userEvent.setup();
    const [pendingHandler, deferred] = draftsListPending();
    server.use(pendingHandler);

    renderApp();
    await screen.findByText("No tasks yet");
    await user.click(screen.getByRole("button", { name: /^new task$/i }));
    expect(await screen.findByText(/loading drafts/i)).toBeInTheDocument();

    deferred.resolve(HttpResponse.json({ drafts: [] }));
  });

  it("shows home entry hint when drafts fail and opens fresh create form", async () => {
    const user = userEvent.setup();
    server.use(
      listCursorModelsOk(),
      http.get("/task-drafts", () =>
        HttpResponse.json({ error: "drafts unavailable" }, { status: 500 }),
      ),
    );

    renderApp();
    await screen.findByText("No tasks yet");
    await user.click(screen.getByRole("button", { name: /^new task$/i }));

    expect(await screen.findByRole("dialog", { name: /^new task$/i })).toBeInTheDocument();
    const draftsHintAlert = await screen.findByRole("alert");
    expect(draftsHintAlert).toHaveTextContent(
      /saved drafts are unavailable right now/i,
    );
    expect(
      screen.getByRole("button", { name: /retry loading drafts/i }),
    ).toBeInTheDocument();
  });

  it("retries draft loading from home entry hint and opens draft picker when available", async () => {
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
              name: "Recovered draft",
              created_at: "2026-04-07T10:00:00Z",
              updated_at: "2026-04-07T10:05:00Z",
            },
          ],
        });
      }),
    );

    renderApp();
    await screen.findByText("No tasks yet");
    await user.click(screen.getByRole("button", { name: /^new task$/i }));
    await user.click(screen.getByRole("button", { name: /retry loading drafts/i }));

    expect(
      await screen.findByRole("heading", { name: /resume a draft or start fresh/i }),
    ).toBeInTheDocument();
  });
});
