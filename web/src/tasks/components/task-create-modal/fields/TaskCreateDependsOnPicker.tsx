import { useQuery } from "@tanstack/react-query";
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import { listTasks } from "@/api";
import { Modal } from "@/shared/Modal";
import { FieldLabel } from "@/shared/FieldLabel";
import { taskQueryKeys } from "@/tasks/task-query";
import type { Task } from "@/types";

type Props = {
  /**
   * Project the new task is being scoped to. The picker only surfaces
   * tasks that share this `project_id` — picking dependencies across
   * project boundaries is not a use case we support today (cross-project
   * dependency wiring belongs in the detail page after both tasks exist).
   * Empty string means "no project chosen" and the picker reads as
   * disabled chrome with a one-line nudge.
   */
  projectId: string;
  selected: string[];
  onChange: (next: string[]) => void;
  disabled: boolean;
};

const MAX_TYPEAHEAD_RESULTS = 8;
const TYPEAHEAD_BLUR_DELAY_MS = 120;

function shortId(id: string): string {
  return id.length > 8 ? id.slice(0, 8) : id;
}

function filterTypeaheadCandidates(
  projectTasks: Task[],
  query: string,
  selectedSet: Set<string>,
  maxResults: number,
): Task[] {
  const q = query.trim().toLowerCase();
  const candidates = projectTasks.filter((t) => !selectedSet.has(t.id));
  if (!q) return candidates.slice(0, maxResults);
  const hits: Task[] = [];
  for (const t of candidates) {
    if (
      t.title.toLowerCase().includes(q) ||
      t.id.toLowerCase().startsWith(q)
    ) {
      hits.push(t);
      if (hits.length >= maxResults) break;
    }
  }
  return hits;
}

function filterBrowseCandidates(
  projectTasks: Task[],
  browseQuery: string,
): Task[] {
  const q = browseQuery.trim().toLowerCase();
  if (!q) return projectTasks;
  return projectTasks.filter(
    (t) =>
      t.title.toLowerCase().includes(q) || t.id.toLowerCase().includes(q),
  );
}

function buildDependsOnHelperCopy(
  hasProject: boolean,
  isLoading: boolean,
  projectTaskCount: number,
): string {
  if (!hasProject) return "Pick a project first to add dependencies.";
  if (isLoading) return "Loading project tasks…";
  if (projectTaskCount === 0) return "No tasks exist in this project yet.";
  return "Other tasks that must complete before the agent picks this one up.";
}

function TaskCreateDependsOnTypeaheadList({
  listboxId,
  typeaheadResults,
  projectTaskCount,
  onSelect,
}: {
  listboxId: string;
  typeaheadResults: Task[];
  projectTaskCount: number;
  onSelect: (id: string) => void;
}) {
  return (
    <ul
      id={listboxId}
      role="listbox"
      className="task-create-deps__list"
      aria-label="Matching tasks"
    >
      {typeaheadResults.map((t) => (
        <li key={t.id} role="option" aria-selected="false">
          <button
            type="button"
            className="task-create-deps__option"
            // `mousedown` (not `click`) so the action lands before
            // the input fires its `blur`, otherwise the deferred
            // close races the click and swallows it.
            onMouseDown={(e) => {
              e.preventDefault();
              onSelect(t.id);
            }}
          >
            <span className="task-create-deps__option-title">
              {t.title || "(untitled task)"}
            </span>
            <span className="task-create-deps__option-meta">{shortId(t.id)}</span>
          </button>
        </li>
      ))}
      {typeaheadResults.length === 0 ? (
        <li className="task-create-deps__option task-create-deps__option--empty">
          {projectTaskCount === 0
            ? "No tasks exist in this project yet."
            : "No tasks match."}
        </li>
      ) : null}
    </ul>
  );
}

function TaskCreateDependsOnSelectedChips({
  selected,
  labelLookup,
  disabled,
  onRemove,
}: {
  selected: string[];
  labelLookup: Map<string, string>;
  disabled: boolean;
  onRemove: (id: string) => void;
}) {
  if (selected.length === 0) return null;

  return (
    <ul className="task-create-deps__chips" aria-label="Selected dependencies">
      {selected.map((id) => (
        <li key={id}>
          <button
            type="button"
            className="task-create-deps__chip"
            onClick={() => onRemove(id)}
            disabled={disabled}
            aria-label={`Remove dependency ${labelLookup.get(id) ?? shortId(id)}`}
          >
            <span className="task-create-deps__chip-title">
              {labelLookup.get(id) ?? shortId(id)}
            </span>
            <span className="task-create-deps__chip-remove" aria-hidden="true">
              ×
            </span>
          </button>
        </li>
      ))}
    </ul>
  );
}

function TaskCreateDependsOnSearchRow({
  inputId,
  listboxId,
  hasProject,
  listOpen,
  query,
  inputDisabled,
  projectTaskCount,
  onQueryChange,
  onFocus,
  onBlur,
  onKeyDown,
  onBrowseOpen,
}: {
  inputId: string;
  listboxId: string;
  hasProject: boolean;
  listOpen: boolean;
  query: string;
  inputDisabled: boolean;
  projectTaskCount: number;
  onQueryChange: (value: string) => void;
  onFocus: () => void;
  onBlur: () => void;
  onKeyDown: (e: KeyboardEvent<HTMLInputElement>) => void;
  onBrowseOpen: () => void;
}) {
  return (
    <div className="task-create-deps__row">
      <input
        id={inputId}
        type="text"
        className="input task-create-deps__search"
        autoComplete="off"
        role="combobox"
        aria-expanded={listOpen && hasProject}
        aria-controls={listboxId}
        aria-autocomplete="list"
        placeholder={
          hasProject ? "Search tasks by name…" : "Pick a project first"
        }
        disabled={inputDisabled}
        value={query}
        onChange={(e) => onQueryChange(e.target.value)}
        onFocus={onFocus}
        onBlur={onBlur}
        onKeyDown={onKeyDown}
      />
      <button
        type="button"
        className="secondary task-create-deps__browse-btn"
        onClick={onBrowseOpen}
        disabled={inputDisabled || projectTaskCount === 0}
      >
        Browse
      </button>
    </div>
  );
}

function useTaskCreateDependsOnPickerState({
  projectId,
  selected,
  onChange,
  disabled,
}: Props) {
  const hasProject = projectId.trim().length > 0;
  const [query, setQuery] = useState("");
  const [listOpen, setListOpen] = useState(false);
  const [browseOpen, setBrowseOpen] = useState(false);
  const [browseQuery, setBrowseQuery] = useState("");
  const blurTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (blurTimerRef.current) clearTimeout(blurTimerRef.current);
    };
  }, []);

  const tasksQuery = useQuery({
    queryKey: taskQueryKeys.list({ limit: 200, offset: 0 }),
    queryFn: ({ signal }) => listTasks(200, 0, { signal }),
    enabled: hasProject,
    staleTime: 30_000,
  });

  const projectTasks = useMemo(() => {
    if (!hasProject) return [] as Task[];
    return (tasksQuery.data?.tasks ?? []).filter(
      (t) => t.project_id === projectId,
    );
  }, [hasProject, projectId, tasksQuery.data?.tasks]);

  const labelLookup = useMemo(() => {
    const m = new Map<string, string>();
    for (const t of projectTasks) m.set(t.id, t.title);
    return m;
  }, [projectTasks]);

  const selectedSet = useMemo(() => new Set(selected), [selected]);

  const typeaheadResults = useMemo(
    () =>
      filterTypeaheadCandidates(
        projectTasks,
        query,
        selectedSet,
        MAX_TYPEAHEAD_RESULTS,
      ),
    [projectTasks, query, selectedSet],
  );

  const browseResults = useMemo(
    () => filterBrowseCandidates(projectTasks, browseQuery),
    [projectTasks, browseQuery],
  );

  const inputDisabled = disabled || !hasProject;

  const addId = useCallback(
    (id: string) => {
      if (selectedSet.has(id)) return;
      onChange([...selected, id]);
    },
    [onChange, selected, selectedSet],
  );

  const removeId = useCallback(
    (id: string) => {
      if (!selectedSet.has(id)) return;
      onChange(selected.filter((s) => s !== id));
    },
    [onChange, selected, selectedSet],
  );

  const toggleId = useCallback(
    (id: string) => {
      if (selectedSet.has(id)) removeId(id);
      else addId(id);
    },
    [addId, removeId, selectedSet],
  );

  const handleSelectFromTypeahead = useCallback(
    (id: string) => {
      addId(id);
      setQuery("");
      setListOpen(true);
    },
    [addId],
  );

  const handleInputFocus = useCallback(() => {
    if (blurTimerRef.current) {
      clearTimeout(blurTimerRef.current);
      blurTimerRef.current = null;
    }
    setListOpen(true);
  }, []);

  const handleInputBlur = useCallback(() => {
    // Defer closing the listbox so a click on a result still fires its
    // `mousedown -> blur -> click` sequence before the listbox unmounts.
    blurTimerRef.current = setTimeout(() => {
      setListOpen(false);
      blurTimerRef.current = null;
    }, TYPEAHEAD_BLUR_DELAY_MS);
  }, []);

  const handleInputKeyDown = useCallback(
    (e: KeyboardEvent<HTMLInputElement>) => {
      if (e.key === "Escape" && listOpen) {
        e.preventDefault();
        setListOpen(false);
        return;
      }
      if (e.key === "Enter" && listOpen && typeaheadResults.length > 0) {
        e.preventDefault();
        handleSelectFromTypeahead(typeaheadResults[0].id);
      }
    },
    [handleSelectFromTypeahead, listOpen, typeaheadResults],
  );

  const handleQueryChange = useCallback((value: string) => {
    setQuery(value);
    setListOpen(true);
  }, []);

  const helperCopy = buildDependsOnHelperCopy(
    hasProject,
    tasksQuery.isLoading,
    projectTasks.length,
  );

  return {
    hasProject,
    query,
    listOpen,
    browseOpen,
    browseQuery,
    projectTasks,
    labelLookup,
    selectedSet,
    typeaheadResults,
    browseResults,
    inputDisabled,
    helperCopy,
    removeId,
    toggleId,
    handleSelectFromTypeahead,
    handleInputFocus,
    handleInputBlur,
    handleInputKeyDown,
    handleQueryChange,
    setBrowseOpen,
    setBrowseQuery,
  };
}

function TaskCreateDependsOnBrowseModal({
  browseTitleId,
  browseQuery,
  browseResults,
  selectedSet,
  selectedCount,
  disabled,
  onBrowseQueryChange,
  onClose,
  onToggle,
}: {
  browseTitleId: string;
  browseQuery: string;
  browseResults: Task[];
  selectedSet: Set<string>;
  selectedCount: number;
  disabled: boolean;
  onBrowseQueryChange: (value: string) => void;
  onClose: () => void;
  onToggle: (id: string) => void;
}) {
  return (
    <Modal
      onClose={onClose}
      labelledBy={browseTitleId}
      stack="nested"
      lockBodyScroll={false}
    >
      <section className="panel task-create-deps-browse">
        <header className="task-create-deps-browse__header">
          <h3 id={browseTitleId} className="task-create-deps-browse__title">
            Project tasks
          </h3>
          <p className="task-create-deps-browse__lede">
            Toggle the tasks this one should wait for.
          </p>
        </header>
        <div className="task-create-deps-browse__search">
          <input
            type="text"
            className="input"
            placeholder="Search tasks…"
            value={browseQuery}
            onChange={(e) => onBrowseQueryChange(e.target.value)}
            aria-label="Filter project tasks"
            autoFocus
          />
        </div>
        {browseResults.length > 0 ? (
          <ul className="task-create-deps-browse__list">
            {browseResults.map((t) => {
              const checked = selectedSet.has(t.id);
              return (
                <li key={t.id} className="task-create-deps-browse__item">
                  <label className="task-create-deps-browse__row">
                    <input
                      type="checkbox"
                      className="task-create-deps-browse__check"
                      checked={checked}
                      onChange={() => onToggle(t.id)}
                      disabled={disabled}
                    />
                    <span className="task-create-deps-browse__title-cell">
                      <span className="task-create-deps-browse__row-title">
                        {t.title || "(untitled task)"}
                      </span>
                      <span className="task-create-deps-browse__row-meta">
                        {shortId(t.id)} · {t.status}
                      </span>
                    </span>
                  </label>
                </li>
              );
            })}
          </ul>
        ) : (
          <p className="task-create-deps-browse__empty">No tasks match.</p>
        )}
        <footer className="task-create-deps-browse__footer">
          <span className="task-create-deps-browse__count">
            {selectedCount === 1
              ? "1 dependency selected"
              : `${selectedCount} dependencies selected`}
          </span>
          <button
            type="button"
            className="task-create-deps-browse__done"
            onClick={onClose}
          >
            Done
          </button>
        </footer>
      </section>
    </Modal>
  );
}

export function TaskCreateDependsOnPicker({
  projectId,
  selected,
  onChange,
  disabled,
}: Props) {
  const inputId = useId();
  const listboxId = useId();
  const browseTitleId = useId();
  const picker = useTaskCreateDependsOnPickerState({
    projectId,
    selected,
    onChange,
    disabled,
  });

  return (
    <div className="task-create-scheduling__field task-create-deps">
      <FieldLabel htmlFor={inputId}>Depends on</FieldLabel>
      <TaskCreateDependsOnSearchRow
        inputId={inputId}
        listboxId={listboxId}
        hasProject={picker.hasProject}
        listOpen={picker.listOpen}
        query={picker.query}
        inputDisabled={picker.inputDisabled}
        projectTaskCount={picker.projectTasks.length}
        onQueryChange={picker.handleQueryChange}
        onFocus={picker.handleInputFocus}
        onBlur={picker.handleInputBlur}
        onKeyDown={picker.handleInputKeyDown}
        onBrowseOpen={() => picker.setBrowseOpen(true)}
      />

      {picker.listOpen && picker.hasProject ? (
        <TaskCreateDependsOnTypeaheadList
          listboxId={listboxId}
          typeaheadResults={picker.typeaheadResults}
          projectTaskCount={picker.projectTasks.length}
          onSelect={picker.handleSelectFromTypeahead}
        />
      ) : null}

      <TaskCreateDependsOnSelectedChips
        selected={selected}
        labelLookup={picker.labelLookup}
        disabled={disabled}
        onRemove={picker.removeId}
      />

      <p className="hint">{picker.helperCopy}</p>

      {picker.browseOpen ? (
        <TaskCreateDependsOnBrowseModal
          browseTitleId={browseTitleId}
          browseQuery={picker.browseQuery}
          browseResults={picker.browseResults}
          selectedSet={picker.selectedSet}
          selectedCount={selected.length}
          disabled={disabled}
          onBrowseQueryChange={picker.setBrowseQuery}
          onClose={() => picker.setBrowseOpen(false)}
          onToggle={picker.toggleId}
        />
      ) : null}
    </div>
  );
}
