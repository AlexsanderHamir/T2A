import path from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineWorkspace } from "vitest/config";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const appIncludes = ["src/app/**/*.test.tsx"];
const taskPagesIncludes = ["src/tasks/pages/**/*.test.tsx"];
const taskCreateIncludes = [
  "src/tasks/create/**/*.test.tsx",
  "!src/tasks/create/hooks/**",
];
const settingsIncludes = ["src/settings/SettingsPage.test.tsx"];
const projectsIncludes = [
  "src/projects/ProjectListPage.test.tsx",
  "src/projects/ProjectDetailPage.test.tsx",
];
const worktreesIncludes = ["src/worktrees/WorktreesPage.test.tsx"];

const fullAppIncludes = [
  ...appIncludes,
  ...taskPagesIncludes,
  ...taskCreateIncludes,
  ...settingsIncludes,
  ...projectsIncludes,
  ...worktreesIncludes,
];

const sharedTest = {
  environment: "jsdom" as const,
  setupFiles: ["./src/test/setup.ts"],
  restoreMocks: true,
  unstubGlobals: true,
};

const sharedVite = {
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
};

const fullAppTest = {
  ...sharedTest,
  testTimeout: 15_000,
};

export default defineWorkspace([
  {
    ...sharedVite,
    test: {
      ...sharedTest,
      name: "unit",
      include: ["src/**/*.test.ts"],
    },
  },
  {
    ...sharedVite,
    test: {
      ...sharedTest,
      name: "components",
      include: ["src/**/*.test.tsx"],
      exclude: fullAppIncludes,
    },
  },
  {
    ...sharedVite,
    test: {
      ...fullAppTest,
      name: "app",
      include: appIncludes,
    },
  },
  {
    ...sharedVite,
    test: {
      ...fullAppTest,
      name: "task-pages",
      include: taskPagesIncludes,
    },
  },
  {
    ...sharedVite,
    test: {
      ...fullAppTest,
      name: "task-create",
      include: taskCreateIncludes,
    },
  },
  {
    ...sharedVite,
    test: {
      ...fullAppTest,
      name: "settings",
      include: settingsIncludes,
    },
  },
  {
    ...sharedVite,
    test: {
      ...fullAppTest,
      name: "projects",
      include: projectsIncludes,
    },
  },
  {
    ...sharedVite,
    test: {
      ...fullAppTest,
      name: "worktrees",
      include: worktreesIncludes,
    },
  },
]);
