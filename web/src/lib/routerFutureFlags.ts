import type { BrowserRouterProps } from "react-router-dom";

/**
 * Opt into React Router v7 behavior early (see upgrade guide). Centralized so
 * {@link BrowserRouter}, {@link MemoryRouter} in tests, and prod stay aligned.
 */
export const ROUTER_FUTURE_FLAGS: NonNullable<BrowserRouterProps["future"]> = {
  v7_startTransition: true,
  v7_relativeSplatPath: true,
};
