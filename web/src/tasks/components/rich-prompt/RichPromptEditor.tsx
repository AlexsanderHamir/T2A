import { EditorContent, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { RepoFileMention } from "../../extensions/repoFileMention";
import {
  RepoFileSuggestion,
  type RepoFileSuggestionOptions,
} from "../../extensions/repoFileSuggestion";
import { ProjectContextMention } from "../../extensions/projectContextMention";
import {
  ProjectContextSuggestion,
  type ProjectContextPickedPayload,
  type ProjectContextSuggestionOptions,
} from "../../extensions/projectContextSuggestion";
import { validateRepoRange } from "../../../api";
import { useDelayedTrue } from "@/lib/useDelayedTrue";
import {
  looksLikeStoredHtml,
  plainTextToInitialHtml,
} from "../../task-prompt";
import { MentionRangePanel } from "./MentionRangePanel";
import { RichPromptMenuBar } from "./RichPromptMenuBar";
import { RichPromptRepoHints } from "./RichPromptRepoHints";
import { useRepoWorkspaceProbe } from "./useRepoWorkspaceProbe";
import { Modal } from "@/shared/Modal";
import { ProjectContextChoiceDialog } from "@/projects/ProjectContextChoiceDialog";
import {
  expandProjectContextSelection,
  mergeProjectContextSelection,
  selectedProjectContextItems,
  type ProjectContextAddMode,
} from "@/projects/projectContextRefs";
import type { ProjectContextEdge, ProjectContextItem } from "@/types";
import { ProjectReferencesBlock } from "./ProjectReferencesBlock";

const EMPTY_CONTEXT_ITEMS: ProjectContextItem[] = [];
const EMPTY_CONTEXT_EDGES: ProjectContextEdge[] = [];
const EMPTY_SELECTED_IDS: string[] = [];

export type RichPromptEditorProjectContextProps = {
  /**
   * All project context items available for the active project. When empty,
   * the `#` suggestion plugin still opens but renders an empty state instead
   * of swallowing the trigger.
   */
  items: ProjectContextItem[];
  /**
   * Project context edges (`source -> target`). Used by the choice dialog to
   * preview how many descendants would be added when the operator picks
   * "Reference this node and its children".
   */
  edges: ProjectContextEdge[];
  /** IDs already on the task. The REFERENCES block renders one row per id. */
  selectedIds: string[];
  /**
   * Replace the selected ids. The editor calls this through the shared
   * `mergeProjectContextSelection`, so callers should not dedupe again.
   */
  onSelectedIdsChange: (ids: string[]) => void;
};

type Props = {
  id: string;
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  placeholder?: string;
  /**
   * When provided, the editor wires the `#` project context suggestion plugin
   * and renders the read-only REFERENCES block above the editable content.
   * Omit on surfaces where project context does not apply (e.g. project edge
   * notes) so behaviour stays unchanged.
   */
  projectContext?: RichPromptEditorProjectContextProps;
};

/** Rich initial prompt (TipTap) with @ file suggestions when the workspace repo (app_settings.repo_root) is set, plus optional `#` project-context mentions. */
export function RichPromptEditor({
  id,
  value,
  onChange,
  disabled,
  placeholder,
  projectContext,
}: Props) {
  const workspaceProbe = useRepoWorkspaceProbe();
  const [fileSearchUnavailable, setFileSearchUnavailable] = useState(false);
  const [fileSuggestBusy, setFileSuggestBusy] = useState(false);
  const [pendingInsert, setPendingInsert] = useState<{
    insertAt: number;
    path: string;
  } | null>(null);
  const [rangeWarning, setRangeWarning] = useState<string | null>(null);
  const lastEmittedHtml = useRef<string | null>(null);

  // Project context state -------------------------------------------------
  const projectItems = projectContext?.items ?? EMPTY_CONTEXT_ITEMS;
  const projectEdges = projectContext?.edges ?? EMPTY_CONTEXT_EDGES;
  const selectedProjectIds = projectContext?.selectedIds ?? EMPTY_SELECTED_IDS;
  const onProjectIdsChange = projectContext?.onSelectedIdsChange;

  // Ref so the suggestion plugin (configured once at mount) can read the
  // freshest items list without rebuilding the TipTap editor on every render.
  // `null` means "no project context wiring on this surface" — the # plugin
  // surfaces the empty-state copy in that case.
  const projectItemsRef = useRef<ProjectContextItem[] | null>(
    projectContext != null ? projectItems : null,
  );
  useEffect(() => {
    projectItemsRef.current = projectContext != null ? projectItems : null;
  }, [projectContext, projectItems]);

  const [pendingProjectChoice, setPendingProjectChoice] = useState<{
    item: ProjectContextItem;
    insertAt: number | null;
  } | null>(null);

  const onFilePicked = useCallback(
    (payload: { insertAt: number; path: string }) => {
      setPendingInsert({ insertAt: payload.insertAt, path: payload.path });
      setRangeWarning(null);
    },
    [],
  );

  const repoOpts = useMemo<RepoFileSuggestionOptions>(
    () => ({
      onRepoUnavailable: () => setFileSearchUnavailable(true),
      onRepoAvailable: () => setFileSearchUnavailable(false),
      onSuggestFetchChange: setFileSuggestBusy,
      onFilePicked,
    }),
    [onFilePicked],
  );

  const projectContextEnabled = projectContext != null;

  const onProjectContextPicked = useCallback(
    (payload: ProjectContextPickedPayload) => {
      setPendingProjectChoice({
        item: payload.item,
        insertAt: payload.insertAt,
      });
    },
    [],
  );

  const projectSuggestionOpts = useMemo<ProjectContextSuggestionOptions>(
    () => ({
      // Read from the ref so the items closure stays stable as the underlying
      // list updates (the suggestion plugin is configured once when the
      // editor mounts; we don't want the items provider to capture a stale
      // snapshot).
      getItems: () => projectItemsRef.current,
      onContextPicked: onProjectContextPicked,
    }),
    [onProjectContextPicked],
  );

  const extensions = useMemo(
    () => [
      StarterKit.configure({
        heading: { levels: [2, 3, 4] },
      }),
      Placeholder.configure({
        placeholder: placeholder ?? "",
      }),
      RepoFileMention,
      RepoFileSuggestion.configure(repoOpts),
      ProjectContextMention,
      ProjectContextSuggestion.configure(projectSuggestionOpts),
    ],
    [placeholder, repoOpts, projectSuggestionOpts],
  );

  const editor = useEditor({
    extensions,
    content: "<p></p>",
    editable: !disabled,
    editorProps: {
      attributes: {
        class: "rich-prompt-editor",
        id,
        "aria-labelledby": `${id}-label`,
      },
    },
    onUpdate: ({ editor: ed }) => {
      const html = ed.getHTML();
      lastEmittedHtml.current = html;
      onChange(html);
    },
  });

  useEffect(() => {
    editor?.setEditable(!disabled);
  }, [editor, disabled]);

  useEffect(() => {
    if (!editor) return;
    if (value === lastEmittedHtml.current) return;
    const next = looksLikeStoredHtml(value)
      ? value
      : plainTextToInitialHtml(value);
    editor.commands.setContent(next, { emitUpdate: false });
    lastEmittedHtml.current = next;
    setPendingInsert(null);
    setPendingProjectChoice(null);
  }, [editor, value]);

  const probeDone = workspaceProbe !== "pending";

  const showRepoMisconfigHint =
    probeDone &&
    (workspaceProbe.state === "unavailable" ||
      workspaceProbe.state === "broken" ||
      (workspaceProbe.state === "available" && fileSearchUnavailable));

  const showRepoUnknownHint = probeDone && workspaceProbe.state === "unknown";
  const rangeModalTitleId = `${id}-mention-range-modal-title`;
  const rangeModalDescId = `${id}-mention-range-modal-desc`;

  /** Avoid flashing “Searching…” when /repo/search returns in a few ms (useDelayedTrue drops immediately when idle). */
  const fileSearchLoadingEligible =
    probeDone &&
    workspaceProbe.state === "available" &&
    fileSuggestBusy &&
    !fileSearchUnavailable;
  const showFileSearchSpinner = useDelayedTrue(fileSearchLoadingEligible, 300);

  const insertPathOnly = () => {
    if (!editor || !pendingInsert) return;
    const { insertAt, path } = pendingInsert;
    editor
      .chain()
      .focus()
      .insertContentAt(insertAt, [
        { type: "repoFileMention", attrs: { path } },
        { type: "text", text: " " },
      ])
      .run();
    setPendingInsert(null);
  };

  const insertWithRange = async (startLine: number, endLine: number) => {
    if (!editor || !pendingInsert) return;
    const { insertAt, path } = pendingInsert;
    const a = startLine;
    const b = endLine;
    setRangeWarning(null);
    const res = await validateRepoRange(path, a, b);
    if (res === null) {
      editor
        .chain()
        .focus()
        .insertContentAt(insertAt, [
          {
            type: "repoFileMention",
            attrs: { path, lineStart: a, lineEnd: b },
          },
          { type: "text", text: " " },
        ])
        .run();
      setPendingInsert(null);
      return;
    }
    if (!res.ok) {
      setRangeWarning(
        res.warning ??
          "That line range is not valid for this file (check line numbers).",
      );
      return;
    }
    editor
      .chain()
      .focus()
      .insertContentAt(insertAt, [
        {
          type: "repoFileMention",
          attrs: { path, lineStart: a, lineEnd: b },
        },
        { type: "text", text: " " },
      ])
      .run();
    setPendingInsert(null);
  };

  const insertProjectContextChip = useCallback(
    (item: ProjectContextItem, insertAt: number | null) => {
      if (!editor) return;
      const chip = {
        type: "projectContextMention",
        attrs: { id: item.id, title: item.title || "" },
      } as const;
      const text = { type: "text", text: " " } as const;
      if (insertAt == null) {
        // Came from the "Choose context" chooser, not the `#` plugin —
        // append the chip to the current cursor instead of jumping to a
        // stale document position from a different surface.
        editor.chain().focus().insertContent([chip, text]).run();
        return;
      }
      editor.chain().focus().insertContentAt(insertAt, [chip, text]).run();
    },
    [editor],
  );

  const confirmProjectContextChoice = useCallback(
    (mode: ProjectContextAddMode) => {
      if (!pendingProjectChoice) return;
      const { item, insertAt } = pendingProjectChoice;
      const expanded = expandProjectContextSelection(
        item.id,
        mode,
        projectEdges,
      );
      const merged = mergeProjectContextSelection(
        selectedProjectIds,
        expanded,
      );
      onProjectIdsChange?.(merged);
      // Always insert a chip for the picked node — the descendants flow into
      // the REFERENCES block but we only insert a single chip at the cursor
      // so the prompt body stays readable.
      insertProjectContextChip(item, insertAt);
      setPendingProjectChoice(null);
    },
    [
      pendingProjectChoice,
      projectEdges,
      selectedProjectIds,
      onProjectIdsChange,
      insertProjectContextChip,
    ],
  );

  const cancelProjectContextChoice = useCallback(() => {
    setPendingProjectChoice(null);
  }, []);

  const removeSelectedProjectId = useCallback(
    (id: string) => {
      if (!onProjectIdsChange) return;
      const next = selectedProjectIds.filter((existing) => existing !== id);
      if (next.length === selectedProjectIds.length) return;
      onProjectIdsChange(next);
    },
    [onProjectIdsChange, selectedProjectIds],
  );

  const referencesItems = useMemo(
    () => selectedProjectContextItems(projectItems, selectedProjectIds),
    [projectItems, selectedProjectIds],
  );

  return (
    <div className="rich-prompt-wrap">
      <RichPromptMenuBar editor={editor} disabled={disabled} />
      {projectContextEnabled ? (
        <ProjectReferencesBlock
          items={referencesItems}
          disabled={disabled}
          onRemove={onProjectIdsChange ? removeSelectedProjectId : undefined}
        />
      ) : null}
      <EditorContent editor={editor} />
      {pendingInsert ? (
        <Modal
          onClose={() => {
            setPendingInsert(null);
            setRangeWarning(null);
          }}
          labelledBy={rangeModalTitleId}
          describedBy={rangeModalDescId}
          size="wide"
        >
          <section className="panel modal-sheet mention-range-modal">
            <h2 id={rangeModalTitleId}>Insert file reference</h2>
            <p id={rangeModalDescId} className="mention-range-modal-desc muted">
              Review the file, optionally choose a line range, then insert it into
              your prompt.
            </p>
            <MentionRangePanel
              id={id}
              path={pendingInsert.path}
              disabled={disabled}
              rangeWarning={rangeWarning}
              onInsertWithRange={insertWithRange}
              onInsertPathOnly={insertPathOnly}
              onCancel={() => {
                setPendingInsert(null);
                setRangeWarning(null);
              }}
            />
          </section>
        </Modal>
      ) : null}
      {pendingProjectChoice ? (
        <ProjectContextChoiceDialog
          item={pendingProjectChoice.item}
          edges={projectEdges}
          selectedIds={selectedProjectIds}
          onClose={cancelProjectContextChoice}
          onConfirm={confirmProjectContextChoice}
        />
      ) : null}
      <RichPromptRepoHints
        showRepoMisconfigHint={showRepoMisconfigHint}
        workspaceBroken={
          workspaceProbe !== "pending" && workspaceProbe.state === "broken"
        }
        fileSearchFailedWhileAvailable={
          workspaceProbe !== "pending" &&
          workspaceProbe.state === "available" &&
          fileSearchUnavailable
        }
        showRepoUnknownHint={showRepoUnknownHint}
        showFileSearchSpinner={showFileSearchSpinner}
      />
    </div>
  );
}
