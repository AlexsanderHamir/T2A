/**
 * Public import surface for the tasks feature (.cursor/rules/CODE_STANDARDS.mdc).
 * The app shell and other cross-cutting entrypoints should import from `@/tasks`
 * instead of deep paths under `tasks/…`.
 */
export { DeleteConfirmDialog } from "./components/dialogs";
export { TaskChangeModelModal, TaskEditForm } from "./components/task-detail";
export { useTasksApp } from "./hooks/useTasksApp";
export { TaskDetailPage } from "./pages/TaskDetailPage";
export { TaskDraftsPage } from "./pages/TaskDraftsPage";
export { TaskEventDetailPage } from "./pages/TaskEventDetailPage";
export { TaskGraphPage } from "./pages/TaskGraphPage";
export { TaskHome } from "./pages/TaskHome";
