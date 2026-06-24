import { setupServer } from "msw/node";

/** Shared MSW server for Vitest. Tests add handlers via `server.use(...)`. */
export const server = setupServer();
