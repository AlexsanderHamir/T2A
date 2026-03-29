import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

// Dev: browser talks to Vite; /tasks and /events proxy to taskapi (avoids CORS).
const api = process.env.VITE_TASKAPI_ORIGIN ?? "http://127.0.0.1:8080";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/tasks": { target: api, changeOrigin: true },
      "/events": { target: api, changeOrigin: true },
      "/repo": { target: api, changeOrigin: true },
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
    restoreMocks: true,
    unstubGlobals: true,
  },
});
