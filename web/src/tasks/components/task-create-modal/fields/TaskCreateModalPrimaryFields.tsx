import type { RichPromptEditorProjectContextProps } from "../../rich-prompt";
import { TaskComposeFields } from "../../task-compose";

type Props = {
  disabled: boolean;
  title: string;
  onTitleChange: (value: string) => void;
  priority: import("@/types").PriorityChoice;
  onPriorityChange: (value: import("@/types").PriorityChoice) => void;
  prompt: string;
  checklistItems: string[];
  hideComposeChecklist: boolean;
  checklistRequirement?: "optional" | "required";
  onPromptChange: (value: string) => void;
  onAppendChecklistCriterion: (text: string) => void;
  onUpdateChecklistRow: (index: number, text: string) => void;
  onRemoveChecklistRow: (index: number) => void;
  /** Forwarded to the rich prompt editor for `#` mentions and the REFERENCES block. */
  projectContext?: RichPromptEditorProjectContextProps;
};

export function TaskCreateModalPrimaryFields({
  disabled,
  title,
  onTitleChange,
  priority,
  onPriorityChange,
  prompt,
  checklistItems,
  hideComposeChecklist,
  checklistRequirement = "optional",
  onPromptChange,
  onAppendChecklistCriterion,
  onUpdateChecklistRow,
  onRemoveChecklistRow,
  projectContext,
}: Props) {
  return (
    <TaskComposeFields
      idsPrefix="task-new"
      editorKey="create-prompt-modal"
      title={title}
      prompt={prompt}
      priority={priority}
      checklistItems={checklistItems}
      hideChecklist={hideComposeChecklist}
      checklistRequirement={checklistRequirement}
      disabled={disabled}
      onTitleChange={onTitleChange}
      onPromptChange={onPromptChange}
      onPriorityChange={onPriorityChange}
      onAppendChecklistCriterion={onAppendChecklistCriterion}
      onUpdateChecklistRow={onUpdateChecklistRow}
      onRemoveChecklistRow={onRemoveChecklistRow}
      projectContext={projectContext}
    />
  );
}
