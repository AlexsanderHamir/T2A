import type { PriorityChoice, TaskType } from "@/types";
import { TaskComposeFields } from "../../task-compose";
import type { RichPromptEditorProjectContextProps } from "../../rich-prompt";
import { TaskCreateModalDmapSection } from "../dmap/TaskCreateModalDmapSection";
import { TaskCreateModalDmapTitleRow } from "../dmap/TaskCreateModalDmapTitleRow";

type Props = {
  dmapMode: boolean;
  disabled: boolean;
  title: string;
  onTitleChange: (value: string) => void;
  priority: PriorityChoice;
  onPriorityChange: (value: PriorityChoice) => void;
  taskType: TaskType;
  onTaskTypeChange: (value: TaskType) => void;
  dmapCommitLimit: string;
  dmapDomain: string;
  dmapDescription: string;
  onDmapCommitLimitChange: (value: string) => void;
  onDmapDomainChange: (value: string) => void;
  onDmapDescriptionChange: (value: string) => void;
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
  dmapMode,
  disabled,
  title,
  onTitleChange,
  priority,
  onPriorityChange,
  taskType,
  onTaskTypeChange,
  dmapCommitLimit,
  dmapDomain,
  dmapDescription,
  onDmapCommitLimitChange,
  onDmapDomainChange,
  onDmapDescriptionChange,
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
  if (dmapMode) {
    return (
      <>
        <TaskCreateModalDmapTitleRow
          title={title}
          onTitleChange={onTitleChange}
          priority={priority}
          onPriorityChange={onPriorityChange}
          taskType={taskType}
          onTaskTypeChange={onTaskTypeChange}
          disabled={disabled}
        />
        <TaskCreateModalDmapSection
          dmapCommitLimit={dmapCommitLimit}
          dmapDomain={dmapDomain}
          dmapDescription={dmapDescription}
          onDmapCommitLimitChange={onDmapCommitLimitChange}
          onDmapDomainChange={onDmapDomainChange}
          onDmapDescriptionChange={onDmapDescriptionChange}
          disabled={disabled}
        />
      </>
    );
  }

  return (
    <TaskComposeFields
      idsPrefix="task-new"
      editorKey="create-prompt-modal"
      title={title}
      prompt={prompt}
      priority={priority}
      taskType={taskType}
      checklistItems={checklistItems}
      hideChecklist={hideComposeChecklist}
      checklistRequirement={checklistRequirement}
      disabled={disabled}
      onTitleChange={onTitleChange}
      onPromptChange={onPromptChange}
      onPriorityChange={onPriorityChange}
      onTaskTypeChange={onTaskTypeChange}
      onAppendChecklistCriterion={onAppendChecklistCriterion}
      onUpdateChecklistRow={onUpdateChecklistRow}
      onRemoveChecklistRow={onRemoveChecklistRow}
      projectContext={projectContext}
    />
  );
}
