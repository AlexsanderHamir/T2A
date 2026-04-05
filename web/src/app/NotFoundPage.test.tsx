import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "../lib/routerFutureFlags";
import { DEFAULT_DOCUMENT_TITLE } from "../shared/useDocumentTitle";
import { NotFoundPage } from "./NotFoundPage";

describe("NotFoundPage", () => {
  it("sets document title and shows a link home", async () => {
    render(
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <Routes>
          <Route path="/" element={<NotFoundPage />} />
        </Routes>
      </MemoryRouter>,
    );

    expect(
      await screen.findByRole("heading", { name: /^page not found$/i }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(document.title).toBe(`Page not found · ${DEFAULT_DOCUMENT_TITLE}`);
    });
    expect(screen.getByRole("link", { name: /^← all tasks$/i })).toHaveAttribute(
      "href",
      "/",
    );
  });
});
