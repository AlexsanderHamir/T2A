import { CustomSelect } from "../../custom-select";
import {
  TASK_LIST_PRIORITY_FILTER_OPTIONS,
  TASK_LIST_STATUS_FILTER_OPTIONS,
} from "./taskListFilterSelectOptions";

type Props = {
  statusFilter: string;
  onStatusFilterChange: (value: string) => void;
  priorityFilter: string;
  onPriorityFilterChange: (value: string) => void;
  projectFilter?: string;
  projectOptions?: Array<{ id: string; name: string }>;
  onProjectFilterChange?: (value: string) => void;
  titleSearch: string;
  onTitleSearchChange: (value: string) => void;
};

export function TaskListFilters({
  statusFilter,
  onStatusFilterChange,
  priorityFilter,
  onPriorityFilterChange,
  projectFilter = "all",
  projectOptions = [],
  onProjectFilterChange,
  titleSearch,
  onTitleSearchChange,
}: Props) {
  return (
    <div
      className="task-list-filters"
      role="search"
      aria-label="Filter tasks"
    >
      <div className="task-list-filter-field">
        <CustomSelect
          id="task-list-filter-status"
          label="Status"
          compact
          listboxName="Filter by status"
          value={statusFilter}
          options={TASK_LIST_STATUS_FILTER_OPTIONS}
          onChange={onStatusFilterChange}
        />
      </div>
      <div className="task-list-filter-field">
        <CustomSelect
          id="task-list-filter-priority"
          label="Priority"
          compact
          listboxName="Filter by priority"
          value={priorityFilter}
          options={TASK_LIST_PRIORITY_FILTER_OPTIONS}
          onChange={onPriorityFilterChange}
        />
      </div>
      {onProjectFilterChange ? (
        <div className="task-list-filter-field">
          <label htmlFor="task-list-filter-project">Project</label>
          <select
            id="task-list-filter-project"
            value={projectFilter}
            onChange={(e) => onProjectFilterChange(e.target.value)}
          >
            <option value="all">All projects</option>
            <option value="none">No project</option>
            {projectOptions.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}
              </option>
            ))}
          </select>
        </div>
      ) : null}
      <div className="field grow task-list-search-field">
        <label htmlFor="task-list-search-title">Search titles</label>
        <input
          id="task-list-search-title"
          type="search"
          value={titleSearch}
          onChange={(e) => onTitleSearchChange(e.target.value)}
          placeholder="Search by title…"
          autoComplete="off"
        />
      </div>
    </div>
  );
}
