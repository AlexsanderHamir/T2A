import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { Link, MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "../lib/routerFutureFlags";
import { RouteAnnouncer } from "./RouteAnnouncer";

function PageA() {
  useEffect(() => {
    document.title = "Page A · T2A";
  }, []);
  return (
    <>
      <p>Page A</p>
      <Link to="/b">To B</Link>
    </>
  );
}

function PageB() {
  useEffect(() => {
    document.title = "Page B · T2A";
  }, []);
  return <p>Page B</p>;
}

function Harness() {
  return (
    <>
      <Routes>
        <Route path="/" element={<PageA />} />
        <Route path="/b" element={<PageB />} />
      </Routes>
      <RouteAnnouncer />
    </>
  );
}

describe("RouteAnnouncer", () => {
  afterEach(() => {
    document.title = "";
  });

  it("reflects document.title after the active route sets it", async () => {
    render(
      <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={["/"]}>
        <Harness />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(document.querySelector(".route-announcer")).toHaveTextContent(
        "Page A · T2A",
      );
    });
  });

  it("updates when location changes and the new page sets title", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={["/"]}>
        <Harness />
      </MemoryRouter>,
    );

    await screen.findByText("Page A");
    await user.click(screen.getByRole("link", { name: /^to b$/i }));

    await waitFor(() => {
      expect(document.querySelector(".route-announcer")).toHaveTextContent(
        "Page B · T2A",
      );
    });
  });
});
