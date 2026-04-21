import { describe, expect, it } from "vitest";
import { CustomSelect, isCustomSelectHeader } from "./custom-select";
import { DraftResumeModal } from "./draft-resume";
import { DeleteConfirmDialog, StreamStatusHint } from "./dialogs";
import { filePreviewLanguageFromPath } from "./file-preview";
import { MentionRangePanel, RichPromptEditor } from "./rich-prompt";
import { taskCreateModalBusyLabel, TaskCreateModal } from "./task-create-modal";
import { TaskChangeModelModal, TaskDetailHeader, TaskEditForm } from "./task-detail";
import { TaskComposeFields } from "./task-compose";
import { filterTasksForListView, TaskListSection, TaskPager } from "./task-list";
import {
  TaskChecklistSkeletonRows,
  TaskDetailPageSkeleton,
} from "./skeletons";

describe("tasks component barrels", () => {
  it("re-exports primary symbols from each family index", () => {
    // `memo(...)` is an object in modern React; plain function components stay `function`.
    expect(["function", "object"]).toContain(typeof TaskListSection);
    expect(TaskPager).toBeTypeOf("function");
    expect(TaskCreateModal).toBeTypeOf("function");
    expect(CustomSelect).toBeTypeOf("function");
    expect(RichPromptEditor).toBeTypeOf("function");
    expect(MentionRangePanel).toBeTypeOf("function");
    expect(TaskDetailPageSkeleton).toBeTypeOf("function");
    expect(TaskDetailHeader).toBeTypeOf("function");
    expect(TaskEditForm).toBeTypeOf("function");
    expect(TaskChangeModelModal).toBeTypeOf("function");
    expect(TaskComposeFields).toBeTypeOf("function");
    expect(DraftResumeModal).toBeTypeOf("function");
    expect(DeleteConfirmDialog).toBeTypeOf("function");
    expect(StreamStatusHint).toBeTypeOf("function");
  });

  it("re-exports non-UI helpers through the same barrels", () => {
    expect(isCustomSelectHeader).toBeTypeOf("function");
    expect(filterTasksForListView).toBeTypeOf("function");
    expect(taskCreateModalBusyLabel).toBeTypeOf("function");
    expect(TaskChecklistSkeletonRows).toBeTypeOf("function");
    expect(filePreviewLanguageFromPath).toBeTypeOf("function");
  });
});
