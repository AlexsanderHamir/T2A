import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import { DEFAULT_DOCUMENT_TITLE } from "@/shared/useDocumentTitle";
import {
  bootstrapNetworkError,
  tasksListNetworkError,
} from "@/test/handlers/bootstrap";
import {
  taskChecklistEmpty,
  taskEventsEmpty,
  taskGet,
  tasksListEmpty,
} from "@/test/handlers/tasks";
import { repoNotConfigured } from "@/test/handlers/repo";
import {
  appDefaultHandlers,
  renderApp,
  renderAppAt,
  setupAppTest,
} from "@/test/integration/appHarness";
import { server } from "@/test/server";

describe("App routing", () => {
  beforeEach(() => {
    setupAppTest();
    server.use(...appDefaultHandlers());
  });

  it("exposes Hamix wordmark as home link with aria-current on /", async () => {
    renderApp();
    const titleLink = await screen.findByRole("link", { name: /^hamix$/i });
    expect(titleLink).toHaveAttribute("href", "/");
    expect(titleLink).toHaveAttribute("aria-current", "page");
  });

  it("navigates home when Hamix wordmark is used from a task route", async () => {
    const user = userEvent.setup();
    server.use(
      taskGet("h1", { id: "h1", title: "Home link task" }),
      taskChecklistEmpty("h1"),
      taskEventsEmpty("h1"),
    );

    renderAppAt(["/tasks/h1"]);

    await screen.findByRole("heading", { name: /^home link task$/i });
    const titleLink = screen.getByRole("link", { name: /^hamix$/i });
    expect(titleLink).not.toHaveAttribute("aria-current");

    await user.click(titleLink);
    expect(await screen.findByText("No tasks yet")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /^hamix$/i })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("shows not found for unknown routes", async () => {
    renderAppAt(["/no-such-page"]);

    expect(
      await screen.findByRole("heading", { name: /^page not found$/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /^all tasks$/i }),
    ).toHaveAttribute("href", "/");
  });

  it("renders heading and empty state after tasks load", async () => {
    renderApp();
    const skip = screen.getByRole("link", { name: /^skip to main content$/i });
    expect(skip).toHaveAttribute("href", "#main-content");
    expect(screen.getByRole("main")).toHaveAttribute("id", "main-content");
    expect(
      await screen.findByRole("link", { name: /^hamix$/i }),
    ).toBeInTheDocument();
    expect(await screen.findByText("No tasks yet")).toBeInTheDocument();
    await waitFor(() => {
      expect(document.title).toBe(DEFAULT_DOCUMENT_TITLE);
    });
    expect(document.querySelector(".route-announcer")).toHaveTextContent(
      DEFAULT_DOCUMENT_TITLE,
    );
  });

  it("shows an alert when the initial list request fails", async () => {
    server.use(bootstrapNetworkError(), tasksListNetworkError());

    renderApp();

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(/network|failed|fetch/i);
  });
});
