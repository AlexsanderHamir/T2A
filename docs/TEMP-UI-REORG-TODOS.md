# Temporary TODOs: UI Folder Reorganization

Status: draft working checklist  
Owner: UI refactor pass  
Scope: `web/src/tasks` (focus on component organization)

## 0) Ground Rules

- [x] Keep behavior unchanged (organization-only refactor)
- [x] Move in small batches with green checks between batches
- [x] Avoid broad renames and avoid changing public APIs during moves
- [x] Keep all imports compiling after each batch
- [ ] Remove this file when reorg is complete

## 1) Target Structure

- [x] Create `web/src/tasks/components/task-create-modal/`
- [x] Create `web/src/tasks/components/task-list/`
- [x] Create `web/src/tasks/components/custom-select/`
- [x] Create `web/src/tasks/components/rich-prompt/`
- [x] Create `web/src/tasks/components/skeletons/`
- [x] Keep one-off components in `components/` root only when truly standalone

## 2) Task Create Modal Family

- [x] Move `TaskCreateModal.tsx` into `task-create-modal/`
- [x] Move `TaskCreateModalPrimaryFields.tsx`
- [x] Move `TaskCreateModalDmapTitleRow.tsx`
- [x] Move `TaskCreateModalDmapSection.tsx`
- [x] Move `TaskCreateModalParentField.tsx`
- [x] Move `TaskCreateModalDraftNameField.tsx`
- [x] Move `TaskCreateModalInheritChecklistField.tsx`
- [x] Move `TaskCreateModalPendingSubtasksField.tsx`
- [x] Move `TaskCreateModalEvaluationSummary.tsx`
- [x] Move `TaskCreateModalFooterActions.tsx`
- [x] Move `TaskCreateModalNestedSubtaskModal.tsx`
- [x] Move `useTaskCreateModalNestedDraft.ts`
- [x] Move `taskCreateModalBusyLabel.ts`
- [x] Move `taskCreateModalDmapReady.ts`
- [x] Move/adjust tests: `task-create-modal/TaskCreateModal.test.tsx` (co-located)
- [x] Add `task-create-modal/index.ts` barrel exports

## 3) Task List Family

- [x] Move `TaskListSection.tsx` into `task-list/`
- [x] Move `TaskListSectionHeading.tsx`
- [x] Move `TaskListFilters.tsx`
- [x] Move `TaskListDataTable.tsx`
- [x] Move `TaskListTableSkeleton.tsx`
- [x] Move `taskListClientFilter.ts`
- [x] Move `taskListFilterSelectOptions.ts`
- [x] Move `taskListPagerSummary.ts`
- [x] Move/adjust tests: `task-list/TaskListSection.test.tsx` (co-located)
- [x] Add `task-list/index.ts` barrel exports

## 4) Custom Select Family

- [x] Move `CustomSelect.tsx` into `custom-select/`
- [x] Move `CustomSelectDropdown.tsx`
- [x] Move `CustomSelectRowBody.tsx`
- [x] Move `customSelectModel.ts`
- [x] Move/adjust tests: `custom-select/CustomSelect.test.tsx` (co-located)
- [x] Add `custom-select/index.ts` barrel exports

## 5) Rich Prompt Family

- [x] Move `RichPromptEditor.tsx` into `rich-prompt/`
- [x] Move `RichPromptMenuBar.tsx`
- [x] Move `RichPromptRepoHints.tsx`
- [x] Move `MentionRangePanel.tsx` (and related helpers/tests)
- [x] Keep extension files under `tasks/extensions/` unless explicitly re-scoped
- [x] Add `rich-prompt/index.ts` barrel exports if helpful

## 6) Skeletons Family

- [x] Move `taskLoadingSkeletons.tsx` into `skeletons/` (later renamed `taskSkeletons.tsx`)
- [x] Move `taskLoadingSkeletonChunks.tsx` (later renamed `taskSkeletonChunks.tsx`)
- [x] Update all imports in pages/components
- [x] Consider naming cleanup (`Task*Skeleton` consistency — module names align with `Task*` exports)

## 7) Import and Path Cleanup

- [x] Update relative imports after each move batch
- [x] Prefer local barrels to reduce long relative paths
- [x] Ensure no duplicate files remain in old locations
- [x] Ensure no circular import introduced by barrels (barrels are flat re-exports only)

## 8) Validation Checklist (run after each batch)

- [x] `npx tsc --noEmit` in `web/`
- [x] Targeted tests for moved family (co-located beside implementations)
  - [x] `task-create-modal/TaskCreateModal.test.tsx`
  - [x] `task-list/TaskListSection.test.tsx`
  - [x] `custom-select/CustomSelect.test.tsx`
  - [x] `rich-prompt/MentionRangePanel.test.tsx`
  - [x] `rich-prompt/RichPromptMenuBar.test.tsx`
- [x] Spot-check lint diagnostics for moved files

## 9) Final Cleanup

- [x] Remove temporary compatibility imports if any were added
- [ ] Remove unused exports and dead files
- [x] Verify no stale references in docs (spot-check: no old flat component paths in `docs/`)
- [ ] Delete this temporary file (`docs/TEMP-UI-REORG-TODOS.md`)

## 10) Suggested Commit Batches

- [x] Batch A: create folders + move task-create-modal family
- [x] Batch B: move task-list family
- [x] Batch C: move custom-select family
- [x] Batch D: move rich-prompt family
- [x] Batch E: move skeletons + final import cleanup
