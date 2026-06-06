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

export function TaskCreateDependsOnPicker({
  projectId,
  selected,
  onChange,
  disabled,
}: Props) {
  const inputId = useId();
  const listboxId = useId();
  const browseTitleId = useId();

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

  // Flatten the root forest (root tasks + their nested children) into a
  // single list so the picker can offer subtasks too — depends_on is a
  // task-id wire field and the server happily accepts subtask ids.
  const projectTasks = useMemo(() => {
    if (!hasProject) return [] as Task[];
    const out: Task[] = [];
    const seen = new Set<string>();
    const walk = (node: Task) => {
      if (seen.has(node.id)) return;
      seen.add(node.id);
      if (node.project_id === projectId) out.push(node);
      for (const child of node.children ?? []) walk(child);
    };
    for (const t of tasksQuery.data?.tasks ?? []) walk(t);
    return out;
  }, [hasProject, projectId, tasksQuery.data?.tasks]);

  const labelLookup = useMemo(() => {
    const m = new Map<string, string>();
    for (const t of projectTasks) m.set(t.id, t.title);
    return m;
  }, [projectTasks]);

  const selectedSet = useMemo(() => new Set(selected), [selected]);

  const typeaheadResults = useMemo(() => {
    const q = query.trim().toLowerCase();
    const candidates = projectTasks.filter((t) => !selectedSet.has(t.id));
    if (!q) return candidates.slice(0, MAX_TYPEAHEAD_RESULTS);
    const hits: Task[] = [];
    for (const t of candidates) {
      if (
        t.title.toLowerCase().includes(q) ||
        t.id.toLowerCase().startsWith(q)
      ) {
        hits.push(t);
        if (hits.length >= MAX_TYPEAHEAD_RESULTS) break;
      }
    }
    return hits;
  }, [projectTasks, query, selectedSet]);

  const browseResults = useMemo(() => {
    const q = browseQuery.trim().toLowerCase();
    if (!q) return projectTasks;
    return projectTasks.filter(
      (t) =>
        t.title.toLowerCase().includes(q) || t.id.toLowerCase().includes(q),
    );
  }, [projectTasks, browseQuery]);

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

  function handleSelectFromTypeahead(id: string) {
    addId(id);
    setQuery("");
    setListOpen(true);
  }

  function handleInputFocus() {
    if (blurTimerRef.current) {
      clearTimeout(blurTimerRef.current);
      blurTimerRef.current = null;
    }
    setListOpen(true);
  }

  function handleInputBlur() {
    // Defer closing the listbox so a click on a result still fires its
    // `mousedown -> blur -> click` sequence before the listbox unmounts.
    blurTimerRef.current = setTimeout(() => {
      setListOpen(false);
      blurTimerRef.current = null;
    }, TYPEAHEAD_BLUR_DELAY_MS);
  }

  function handleInputKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Escape" && listOpen) {
      e.preventDefault();
      setListOpen(false);
      return;
    }
    if (e.key === "Enter" && listOpen && typeaheadResults.length > 0) {
      e.preventDefault();
      handleSelectFromTypeahead(typeaheadResults[0].id);
    }
  }

  const helperCopy = !hasProject
    ? "Pick a project first to add dependencies."
    : tasksQuery.isLoading
      ? "Loading project tasks…"
      : projectTasks.length === 0
        ? "No tasks exist in this project yet."
        : "Other tasks that must complete before the agent picks this one up.";

  return (
    <div className="task-create-scheduling__field task-create-deps">
      <FieldLabel htmlFor={inputId}>Depends on</FieldLabel>
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
          onChange={(e) => {
            setQuery(e.target.value);
            setListOpen(true);
          }}
          onFocus={handleInputFocus}
          onBlur={handleInputBlur}
          onKeyDown={handleInputKeyDown}
        />
        <button
          type="button"
          className="secondary task-create-deps__browse-btn"
          onClick={() => setBrowseOpen(true)}
          disabled={inputDisabled || projectTasks.length === 0}
        >
          Browse
        </button>
      </div>

      {listOpen && hasProject ? (
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
                  handleSelectFromTypeahead(t.id);
                }}
              >
                <span className="task-create-deps__option-title">
                  {t.title || "(untitled task)"}
                </span>
                <span className="task-create-deps__option-meta">
                  {shortId(t.id)}
                </span>
              </button>
            </li>
          ))}
          {typeaheadResults.length === 0 ? (
            <li className="task-create-deps__option task-create-deps__option--empty">
              {projectTasks.length === 0
                ? "No tasks exist in this project yet."
                : "No tasks match."}
            </li>
          ) : null}
        </ul>
      ) : null}

      {selected.length > 0 ? (
        <ul className="task-create-deps__chips" aria-label="Selected dependencies">
          {selected.map((id) => (
            <li key={id}>
              <button
                type="button"
                className="task-create-deps__chip"
                onClick={() => removeId(id)}
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
      ) : null}

      <p className="hint">{helperCopy}</p>

      {browseOpen ? (
        <Modal
          onClose={() => setBrowseOpen(false)}
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
                onChange={(e) => setBrowseQuery(e.target.value)}
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
                          onChange={() => toggleId(t.id)}
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
                {selected.length === 1
                  ? "1 dependency selected"
                  : `${selected.length} dependencies selected`}
              </span>
              <button
                type="button"
                className="task-create-deps-browse__done"
                onClick={() => setBrowseOpen(false)}
              >
                Done
              </button>
            </footer>
          </section>
        </Modal>
      ) : null}
    </div>
  );
}

function shortId(id: string): string {
  return id.length > 8 ? id.slice(0, 8) : id;
}
