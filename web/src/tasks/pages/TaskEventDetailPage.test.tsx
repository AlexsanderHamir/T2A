import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DEFAULT_DOCUMENT_TITLE } from "../../shared/useDocumentTitle";
import { requestUrl } from "../../test/requestUrl";
import { TaskEventDetailPage } from "./TaskEventDetailPage";

function renderEventPage(initialPath: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route
            path="/tasks/:taskId/events/:eventSeq"
            element={<TaskEventDetailPage />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("TaskEventDetailPage", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("loads one event and shows type, time, actor, and JSON data", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/t1/events/2") {
        return Response.json({
          task_id: "t1",
          seq: 2,
          at: "2026-03-01T10:00:00.000Z",
          type: "message_added",
          by: "agent",
          data: { from: "a", to: "b" },
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderEventPage("/tasks/t1/events/2");

    expect(
      await screen.findByRole("heading", { name: /event #2/i }),
    ).toBeInTheDocument();
    expect(document.title).toBe(
      `Event #2: Title or message updated · ${DEFAULT_DOCUMENT_TITLE}`,
    );
    expect(screen.getByText("t1")).toBeInTheDocument();
    const pill = document.querySelector(
      "code.task-timeline-type-pill[data-event-type='message_added']",
    );
    expect(pill).not.toBeNull();
    expect(screen.getByText(/data \(json\)/i)).toBeInTheDocument();
    expect(screen.getByText(/"from"/)).toBeInTheDocument();
  });

  it("shows a response form when the event type needs user input", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/t1/events/2") {
        return Response.json({
          task_id: "t1",
          seq: 2,
          at: "2026-03-01T10:00:00.000Z",
          type: "approval_requested",
          by: "agent",
          data: {},
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderEventPage("/tasks/t1/events/2");

    expect(
      await screen.findByRole("heading", { name: /^add a message$/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", { name: /^add a message$/i }),
    ).toBeInTheDocument();
    expect(screen.getByText("Agent needs input")).toHaveAttribute(
      "data-awaiting-response",
      "true",
    );
  });

  it("does not mark awaiting when user has replied (legacy user_response)", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = requestUrl(input);
      if (url === "/tasks/t1/events/2") {
        return Response.json({
          task_id: "t1",
          seq: 2,
          at: "2026-03-01T10:00:00.000Z",
          type: "approval_requested",
          by: "agent",
          data: {},
          user_response: "Approved",
          user_response_at: "2026-03-01T10:05:00.000Z",
        });
      }
      return new Response("not found", { status: 404 });
    });

    renderEventPage("/tasks/t1/events/2");

    await screen.findByRole("heading", { name: /^add a message$/i });
    expect(screen.getByText("You replied — waiting on agent")).not.toHaveAttribute(
      "data-awaiting-response",
    );
    const convo = screen.getByRole("log", {
      name: /conversation on this event/i,
    });
    expect(convo).toHaveTextContent("Approved");
    const sentAt = within(convo).getByRole("time");
    expect(sentAt).toHaveAttribute("dateTime", "2026-03-01T10:05:00.000Z");
    expect(
      screen.getByRole("textbox", { name: /^add a message$/i }),
    ).toHaveValue("");
  });

  it("shows a client-side message when seq is not a positive integer", () => {
    vi.spyOn(globalThis, "fetch");
    renderEventPage("/tasks/t1/events/nope");

    expect(
      screen.getByText(/invalid event sequence/i),
    ).toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled();
  });
});
