/**
 * Public import surface for the tasks feature (.cursor/rules/CODE_STANDARDS.mdc).
 * The app shell and other cross-cutting entrypoints should import from `@/tasks`
 * instead of deep paths under `tasks/…`.
 *
 * TaskDetailPage / TaskCycleDetailPage / TaskEventDetailPage are NOT
 * re-exported here on purpose — they are route-
 * level entry points loaded via React.lazy() in App.tsx. Re-exporting
 * them from this barrel would force Rollup to bundle them into the
 * same chunk that imports the barrel, defeating the code split.
 * Import them directly from "@/tasks/pages/<PageName>" only inside
 * the lazy-loader or tests.
 */
export { AutonomyConfirmDialog, DeleteConfirmDialog } from "./components/dialogs";
export { TaskChangeModelModal } from "./components/task-detail";
export { useTasksApp } from "./hooks/useTasksApp";
export { TaskDraftsPage } from "./pages/TaskDraftsPage";
export { TaskTemplatesPage } from "./pages/TaskTemplatesPage";
export { TaskCreateModalsLayer } from "./pages/TaskCreateModalsLayer";
export { TaskHome } from "./pages/TaskHome";
