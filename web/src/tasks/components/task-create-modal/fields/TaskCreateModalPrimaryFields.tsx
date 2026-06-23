import type { ReactNode } from "react";
import type { ChecklistItemDraft } from "@/types";
import type { RichPromptEditorProjectContextProps } from "../../rich-prompt";
import { TaskComposeFields } from "../../task-compose";

type Props = {
  idsPrefix?: string;
  editorKey?: string;
  disabled: boolean;
  title: string;
  onTitleChange: (value: string) => void;
  priority: import("@/types").PriorityChoice;
  onPriorityChange: (value: import("@/types").PriorityChoice) => void;
  prompt: string;
  checklistItems: ChecklistItemDraft[];
  hideComposeChecklist: boolean;
  checklistRequirement?: "optional" | "required";
  checklistDisabled?: boolean;
  onPromptChange: (value: string) => void;
  onAppendChecklistCriterion: (item: ChecklistItemDraft | string) => void;
  onUpdateChecklistRow: (index: number, item: ChecklistItemDraft) => void;
  onRemoveChecklistRow: (index: number) => void;
  /** Forwarded to the rich prompt editor for `#` mentions and the REFERENCES block. */
  projectContext?: RichPromptEditorProjectContextProps;
  betweenTitleAndPrompt?: ReactNode;
  worktreeId?: string;
};

export function TaskCreateModalPrimaryFields({
  idsPrefix = "task-new",
  editorKey = "create-prompt-modal",
  disabled,
  title,
  onTitleChange,
  priority,
  onPriorityChange,
  prompt,
  checklistItems,
  hideComposeChecklist,
  checklistRequirement = "optional",
  checklistDisabled = false,
  onPromptChange,
  onAppendChecklistCriterion,
  onUpdateChecklistRow,
  onRemoveChecklistRow,
  projectContext,
  betweenTitleAndPrompt,
  worktreeId,
}: Props) {
  return (
    <TaskComposeFields
      idsPrefix={idsPrefix}
      editorKey={editorKey}
      title={title}
      prompt={prompt}
      priority={priority}
      checklistItems={checklistItems}
      hideChecklist={hideComposeChecklist}
      checklistRequirement={checklistRequirement}
      checklistDisabled={checklistDisabled}
      disabled={disabled}
      onTitleChange={onTitleChange}
      onPromptChange={onPromptChange}
      onPriorityChange={onPriorityChange}
      onAppendChecklistCriterion={onAppendChecklistCriterion}
      onUpdateChecklistRow={onUpdateChecklistRow}
      onRemoveChecklistRow={onRemoveChecklistRow}
      projectContext={projectContext}
      betweenTitleAndPrompt={betweenTitleAndPrompt}
      worktreeId={worktreeId}
    />
  );
}
