import path from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Dev: browser talks to Vite; API/observability routes proxy to taskapi (avoids CORS).
const api = process.env.VITE_TASKAPI_ORIGIN ?? "http://127.0.0.1:8080";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) return;
          if (id.includes("@tanstack/react-query")) return "rq";
          if (
            id.includes("@tiptap") ||
            id.includes("prosemirror") ||
            id.includes("tippy.js")
          ) {
            return "editor";
          }
          if (id.includes("react-dom") || id.includes("/react/")) {
            return "react";
          }
        },
      },
    },
  },
  server: {
    proxy: {
      // Document navigations to /tasks/:id send Accept: text/html; serve the SPA instead of proxying to taskapi.
      "/tasks": {
        target: api,
        changeOrigin: true,
        bypass(req) {
          const accept = req.headers.accept ?? "";
          if (accept.includes("text/html")) {
            return "/index.html";
          }
        },
      },
      "/task-drafts": { target: api, changeOrigin: true },
      "/events": { target: api, changeOrigin: true },
      "/repo": { target: api, changeOrigin: true },
      // GET/PATCH /settings + POST /settings/probe-cursor + POST /settings/cancel-current-run.
      // Without this proxy the SettingsPage's GET /settings hits Vite directly and renders "Error: Not Found".
      "/settings": { target: api, changeOrigin: true },
      "/system": { target: api, changeOrigin: true },
      // So the SPA can probe taskapi readiness (workspace repo from app_settings.repo_root) without a full /repo/search walk.
      "/health": { target: api, changeOrigin: true },
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
