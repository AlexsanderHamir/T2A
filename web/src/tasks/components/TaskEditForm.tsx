import type { FormEvent } from "react";
import type { Priority } from "@/types";
import { PrioritySelect } from "./PrioritySelect";
import { RichPromptEditor } from "./RichPromptEditor";

type Props = {
  taskId: string;
  title: string;
  prompt: string;
  priority: Priority;
  saving: boolean;
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onPriorityChange: (p: Priority) => void;
  onSubmit: (e: FormEvent) => void;
  onCancel: () => void;
};

export function TaskEditForm({
  taskId,
  title,
  prompt,
  priority,
  saving,
  onTitleChange,
  onPromptChange,
  onPriorityChange,
  onSubmit,
  onCancel,
}: Props) {
  return (
    <section className="panel">
      <h2>Edit task</h2>
      <form onSubmit={(e) => void onSubmit(e)}>
        <p className="muted stack-tight-zero">
          <code>{taskId}</code>
        </p>
        <div className="row">
          <div className="field grow">
            <label htmlFor="task-edit-title">Title</label>
            <input
              id="task-edit-title"
              value={title}
              onChange={(ev) => onTitleChange(ev.target.value)}
              required
            />
          </div>
          <PrioritySelect
            id="task-edit-priority"
            value={priority}
            onChange={onPriorityChange}
          />
        </div>
        <div className="field grow stack-tight prompt-field-full">
          <label id="task-edit-prompt-label" htmlFor="task-edit-prompt">
            Initial prompt
          </label>
          <RichPromptEditor
            key={taskId}
            id="task-edit-prompt"
            value={prompt}
            onChange={onPromptChange}
            disabled={saving}
            placeholder="Use the toolbar for headings and bold. Type @ to pick a file from the repo."
          />
        </div>
        <div className="row stack-row-actions">
          <button type="submit" disabled={saving}>
            Save
          </button>
          <button
            type="button"
            className="secondary"
            disabled={saving}
            onClick={onCancel}
          >
            Cancel
          </button>
        </div>
      </form>
    </section>
  );
}
