import { describe, expect, it } from "vitest";
import { CustomSelect, isCustomSelectHeader } from "./custom-select";
import { MentionRangePanel, RichPromptEditor } from "./rich-prompt";
import { taskCreateModalBusyLabel, TaskCreateModal } from "./task-create-modal";
import { filterTasksForListView, TaskListSection } from "./task-list";
import {
  TaskChecklistSkeletonRows,
  TaskDetailPageSkeleton,
} from "./skeletons";

describe("tasks component barrels", () => {
  it("re-exports primary symbols from each family index", () => {
    expect(TaskListSection).toBeTypeOf("function");
    expect(TaskCreateModal).toBeTypeOf("function");
    expect(CustomSelect).toBeTypeOf("function");
    expect(RichPromptEditor).toBeTypeOf("function");
    expect(MentionRangePanel).toBeTypeOf("function");
    expect(TaskDetailPageSkeleton).toBeTypeOf("function");
  });

  it("re-exports non-UI helpers through the same barrels", () => {
    expect(isCustomSelectHeader).toBeTypeOf("function");
    expect(filterTasksForListView).toBeTypeOf("function");
    expect(taskCreateModalBusyLabel).toBeTypeOf("function");
    expect(TaskChecklistSkeletonRows).toBeTypeOf("function");
  });
});
