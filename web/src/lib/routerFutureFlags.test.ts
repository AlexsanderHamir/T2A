import { describe, expect, it } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "./routerFutureFlags";

describe("ROUTER_FUTURE_FLAGS", () => {
  it("matches React Router v7 opt-in flags used by BrowserRouter and MemoryRouter in app + tests", () => {
    expect(ROUTER_FUTURE_FLAGS).toEqual({
      v7_startTransition: true,
      v7_relativeSplatPath: true,
    });
  });
});
