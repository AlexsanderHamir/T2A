import type { FormEvent } from "react";
import type { Priority, Status } from "@/types";
import { PrioritySelect } from "./PrioritySelect";
import { RichPromptEditor } from "./RichPromptEditor";
import { StatusSelect } from "./StatusSelect";

type Props = {
  title: string;
  prompt: string;
  status: Status;
  priority: Priority;
  saving: boolean;
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onStatusChange: (s: Status) => void;
  onPriorityChange: (p: Priority) => void;
  onSubmit: (e: FormEvent) => void;
};

export function TaskCreateForm({
  title,
  prompt,
  status,
  priority,
  saving,
  onTitleChange,
  onPromptChange,
  onStatusChange,
  onPriorityChange,
  onSubmit,
}: Props) {
  return (
    <section className="panel">
      <h2>New task</h2>
      <form onSubmit={onSubmit}>
        <div className="row">
          <div className="field grow">
            <label htmlFor="task-new-title">Title</label>
            <input
              id="task-new-title"
              value={title}
              onChange={(ev) => onTitleChange(ev.target.value)}
              placeholder="What should get done?"
              required
            />
          </div>
          <StatusSelect
            id="task-new-status"
            value={status}
            onChange={onStatusChange}
          />
          <PrioritySelect
            id="task-new-priority"
            value={priority}
            onChange={onPriorityChange}
          />
          <button type="submit" disabled={saving}>
            Create
          </button>
        </div>
        <div className="field grow stack-tight prompt-field-full">
          <label id="task-new-prompt-label" htmlFor="task-new-prompt">
            Initial prompt
          </label>
          <RichPromptEditor
            key="create-prompt"
            id="task-new-prompt"
            value={prompt}
            onChange={onPromptChange}
            disabled={saving}
            placeholder="Optional context for an agent… Use the toolbar for headings and bold. Type @ to pick a file from the repo."
          />
        </div>
      </form>
    </section>
  );
}
