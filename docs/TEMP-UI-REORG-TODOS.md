# Temporary TODOs: UI Folder Reorganization

Status: draft working checklist  
Owner: UI refactor pass  
Scope: `web/src/tasks` (focus on component organization)

## 0) Ground Rules

- [ ] Keep behavior unchanged (organization-only refactor)
- [ ] Move in small batches with green checks between batches
- [ ] Avoid broad renames and avoid changing public APIs during moves
- [ ] Keep all imports compiling after each batch
- [ ] Remove this file when reorg is complete

## 1) Target Structure

- [ ] Create `web/src/tasks/components/task-create-modal/`
- [x] Create `web/src/tasks/components/task-list/`
- [x] Create `web/src/tasks/components/custom-select/`
- [x] Create `web/src/tasks/components/rich-prompt/`
- [x] Create `web/src/tasks/components/skeletons/`
- [ ] Keep one-off components in `components/` root only when truly standalone

## 2) Task Create Modal Family

- [ ] Move `TaskCreateModal.tsx` into `task-create-modal/`
- [ ] Move `TaskCreateModalPrimaryFields.tsx`
- [ ] Move `TaskCreateModalDmapTitleRow.tsx`
- [ ] Move `TaskCreateModalDmapSection.tsx`
- [ ] Move `TaskCreateModalParentField.tsx`
- [ ] Move `TaskCreateModalDraftNameField.tsx`
- [ ] Move `TaskCreateModalInheritChecklistField.tsx`
- [ ] Move `TaskCreateModalPendingSubtasksField.tsx`
- [ ] Move `TaskCreateModalEvaluationSummary.tsx`
- [ ] Move `TaskCreateModalFooterActions.tsx`
- [ ] Move `TaskCreateModalNestedSubtaskModal.tsx`
- [ ] Move `useTaskCreateModalNestedDraft.ts`
- [ ] Move `taskCreateModalBusyLabel.ts`
- [ ] Move `taskCreateModalDmapReady.ts`
- [ ] Move/adjust tests: `TaskCreateModal.test.tsx`
- [ ] Add `task-create-modal/index.ts` barrel exports

## 3) Task List Family

- [x] Move `TaskListSection.tsx` into `task-list/`
- [x] Move `TaskListSectionHeading.tsx`
- [x] Move `TaskListFilters.tsx`
- [x] Move `TaskListDataTable.tsx`
- [x] Move `TaskListTableSkeleton.tsx`
- [x] Move `taskListClientFilter.ts`
- [x] Move `taskListFilterSelectOptions.ts`
- [x] Move `taskListPagerSummary.ts`
- [x] Move/adjust tests: `TaskListSection.test.tsx`
- [x] Add `task-list/index.ts` barrel exports

## 4) Custom Select Family

- [x] Move `CustomSelect.tsx` into `custom-select/`
- [x] Move `CustomSelectDropdown.tsx`
- [x] Move `CustomSelectRowBody.tsx`
- [x] Move `customSelectModel.ts`
- [x] Move/adjust tests: `CustomSelect.test.tsx`
- [x] Add `custom-select/index.ts` barrel exports

## 5) Rich Prompt Family

- [x] Move `RichPromptEditor.tsx` into `rich-prompt/`
- [x] Move `RichPromptMenuBar.tsx`
- [x] Move `RichPromptRepoHints.tsx`
- [x] Move `MentionRangePanel.tsx` (and related helpers/tests)
- [ ] Keep extension files under `tasks/extensions/` unless explicitly re-scoped
- [x] Add `rich-prompt/index.ts` barrel exports if helpful

## 6) Skeletons Family

- [x] Move `taskLoadingSkeletons.tsx` into `skeletons/`
- [x] Move `taskLoadingSkeletonChunks.tsx`
- [ ] Update all imports in pages/components
- [ ] Consider naming cleanup (`Task*Skeleton` consistency)

## 7) Import and Path Cleanup

- [ ] Update relative imports after each move batch
- [ ] Prefer local barrels to reduce long relative paths
- [ ] Ensure no duplicate files remain in old locations
- [ ] Ensure no circular import introduced by barrels

## 8) Validation Checklist (run after each batch)

- [x] `npx tsc --noEmit` in `web/`
- [ ] Targeted tests for moved family
  - [ ] `TaskCreateModal.test.tsx`
  - [x] `TaskListSection.test.tsx`
  - [x] `CustomSelect.test.tsx`
  - [x] `MentionRangePanel.test.tsx` (when rich-prompt batch happens)
- [x] Spot-check lint diagnostics for moved files

## 9) Final Cleanup

- [ ] Remove temporary compatibility imports if any were added
- [ ] Remove unused exports and dead files
- [ ] Verify no stale references in docs
- [ ] Delete this temporary file (`docs/TEMP-UI-REORG-TODOS.md`)

## 10) Suggested Commit Batches

- [ ] Batch A: create folders + move task-create-modal family
- [ ] Batch B: move task-list family
- [ ] Batch C: move custom-select family
- [ ] Batch D: move rich-prompt family
- [ ] Batch E: move skeletons + final import cleanup
