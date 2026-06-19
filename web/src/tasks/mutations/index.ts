export { mergePatchIntoTask, patchTaskInList } from "./optimisticTaskDetail";
export type { TaskDetailPatchFields } from "./optimisticTaskDetail";
export {
  beginGuardedTaskWrite,
  cancelQueriesForKeys,
  endGuardedTaskWrite,
  recordOptimisticApplied,
} from "./guardedTaskWrite";
export type { GuardedWriteContext } from "./guardedTaskWrite";
