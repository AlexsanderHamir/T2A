import { describe, expect, it } from "vitest";
import { CustomSelect } from "./custom-select";
import { RichPromptEditor } from "./rich-prompt";
import { TaskCreateModal } from "./task-create-modal";
import { TaskListSection } from "./task-list";
import { TaskDetailPageSkeleton } from "./skeletons";

describe("tasks component barrels", () => {
  it("re-exports primary symbols from each family index", () => {
    expect(TaskListSection).toBeTypeOf("function");
    expect(TaskCreateModal).toBeTypeOf("function");
    expect(CustomSelect).toBeTypeOf("function");
    expect(RichPromptEditor).toBeTypeOf("function");
    expect(TaskDetailPageSkeleton).toBeTypeOf("function");
  });
});
