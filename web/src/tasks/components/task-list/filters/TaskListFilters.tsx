import { isUiFeatureOmitted } from "@/launch/omittedFeatures";
import { CustomSelect } from "../../custom-select";
import {
  TASK_LIST_PRIORITY_FILTER_OPTIONS,
  taskListStatusFilterOptions,
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
  const statusFilterOptions = taskListStatusFilterOptions({
    includeScheduled: !isUiFeatureOmitted("schedule"),
  });
  const projectFilterOptions = [
    { value: "all", label: "All projects" },
    ...projectOptions.map((project) => ({
      value: project.id,
      label: project.name,
    })),
  ];

  return (
    <div
      className="task-list-filters"
      role="search"
      aria-label="Filter tasks"
    >
      <div className="task-list-filters__controls">
        <div className="task-list-filter-field task-list-filter-field--status">
          <CustomSelect
            id="task-list-filter-status"
            label="Status"
            compact
            dropdownVariant="toolbar"
            dropdownMinWidth={240}
            listboxName="Filter by status"
            value={statusFilter}
            options={statusFilterOptions}
            onChange={onStatusFilterChange}
          />
        </div>
        <div className="task-list-filter-field">
          <CustomSelect
            id="task-list-filter-priority"
            label="Priority"
            compact
            dropdownVariant="toolbar"
            dropdownMinWidth={200}
            listboxName="Filter by priority"
            value={priorityFilter}
            options={TASK_LIST_PRIORITY_FILTER_OPTIONS}
            onChange={onPriorityFilterChange}
          />
        </div>
        {onProjectFilterChange ? (
          <div className="task-list-filter-field task-list-filter-field--project">
            <CustomSelect
              id="task-list-filter-project"
              label="Project"
              compact
              dropdownVariant="toolbar"
              dropdownMinWidth={220}
              listboxName="Filter by project"
              value={projectFilter}
              options={projectFilterOptions}
              onChange={onProjectFilterChange}
            />
          </div>
        ) : null}
      </div>
      <div className="field grow task-list-search-field">
        <label htmlFor="task-list-search-title" className="visually-hidden">
          Search titles
        </label>
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
