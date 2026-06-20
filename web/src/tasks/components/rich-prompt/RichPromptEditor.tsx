import { EditorContent, useEditor, type Editor } from "@tiptap/react";
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
import {
  useRepoWorkspaceProbe,
} from "./useRepoWorkspaceProbe";
import type { RepoWorkspaceProbe } from "@/api";
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

type PendingFileInsert = {
  insertAt: number;
  path: string;
};

type PendingProjectChoice = {
  item: ProjectContextItem;
  insertAt: number | null;
};

type RepoHintFlags = {
  showRepoMisconfigHint: boolean;
  workspaceBroken: boolean;
  fileSearchFailedWhileAvailable: boolean;
  showRepoUnknownHint: boolean;
  showFileSearchSpinner: boolean;
};

function buildRichPromptExtensions(
  placeholder: string | undefined,
  repoOpts: RepoFileSuggestionOptions,
  projectSuggestionOpts: ProjectContextSuggestionOptions,
) {
  return [
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
  ];
}

function insertRepoFileMentionAt(
  editor: Editor,
  insertAt: number,
  path: string,
  lineStart?: number,
  lineEnd?: number,
) {
  const attrs =
    lineStart != null && lineEnd != null
      ? { path, lineStart, lineEnd }
      : { path };
  editor
    .chain()
    .focus()
    .insertContentAt(insertAt, [
      { type: "repoFileMention", attrs },
      { type: "text", text: " " },
    ])
    .run();
}

function insertProjectContextChipAt(
  editor: Editor,
  item: ProjectContextItem,
  insertAt: number | null,
) {
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
}

function computeRepoHintFlags(
  workspaceProbe: RepoWorkspaceProbe | "pending",
  fileSearchUnavailable: boolean,
  showFileSearchSpinner: boolean,
): RepoHintFlags {
  const probeDone = workspaceProbe !== "pending";
  const showRepoMisconfigHint =
    probeDone &&
    (workspaceProbe.state === "unavailable" ||
      workspaceProbe.state === "broken" ||
      (workspaceProbe.state === "available" && fileSearchUnavailable));
  const showRepoUnknownHint = probeDone && workspaceProbe.state === "unknown";

  return {
    showRepoMisconfigHint,
    workspaceBroken:
      workspaceProbe !== "pending" && workspaceProbe.state === "broken",
    fileSearchFailedWhileAvailable:
      workspaceProbe !== "pending" &&
      workspaceProbe.state === "available" &&
      fileSearchUnavailable,
    showRepoUnknownHint,
    showFileSearchSpinner,
  };
}

function useRichPromptEditorController({
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
  const [pendingInsert, setPendingInsert] = useState<PendingFileInsert | null>(
    null,
  );
  const [rangeWarning, setRangeWarning] = useState<string | null>(null);
  const lastEmittedHtml = useRef<string | null>(null);

  const projectItems = projectContext?.items ?? EMPTY_CONTEXT_ITEMS;
  const projectEdges = projectContext?.edges ?? EMPTY_CONTEXT_EDGES;
  const selectedProjectIds = projectContext?.selectedIds ?? EMPTY_SELECTED_IDS;
  const onProjectIdsChange = projectContext?.onSelectedIdsChange;

  const projectItemsRef = useRef<ProjectContextItem[] | null>(
    projectContext != null ? projectItems : null,
  );
  useEffect(() => {
    projectItemsRef.current = projectContext != null ? projectItems : null;
  }, [projectContext, projectItems]);

  const [pendingProjectChoice, setPendingProjectChoice] =
    useState<PendingProjectChoice | null>(null);

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
      getItems: () => projectItemsRef.current,
      onContextPicked: onProjectContextPicked,
    }),
    [onProjectContextPicked],
  );

  const extensions = useMemo(
    () =>
      buildRichPromptExtensions(placeholder, repoOpts, projectSuggestionOpts),
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
  const fileSearchLoadingEligible =
    probeDone &&
    workspaceProbe.state === "available" &&
    fileSuggestBusy &&
    !fileSearchUnavailable;
  const showFileSearchSpinner = useDelayedTrue(fileSearchLoadingEligible, 300);

  const repoHints = computeRepoHintFlags(
    workspaceProbe,
    fileSearchUnavailable,
    showFileSearchSpinner,
  );

  const insertPathOnly = useCallback(() => {
    if (!editor || !pendingInsert) return;
    insertRepoFileMentionAt(
      editor,
      pendingInsert.insertAt,
      pendingInsert.path,
    );
    setPendingInsert(null);
  }, [editor, pendingInsert]);

  const insertWithRange = useCallback(
    async (startLine: number, endLine: number) => {
      if (!editor || !pendingInsert) return;
      const { insertAt, path } = pendingInsert;
      setRangeWarning(null);
      const res = await validateRepoRange(path, startLine, endLine);
      if (res === null) {
        insertRepoFileMentionAt(editor, insertAt, path, startLine, endLine);
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
      insertRepoFileMentionAt(editor, insertAt, path, startLine, endLine);
      setPendingInsert(null);
    },
    [editor, pendingInsert],
  );

  const insertProjectContextChip = useCallback(
    (item: ProjectContextItem, insertAt: number | null) => {
      if (!editor) return;
      insertProjectContextChipAt(editor, item, insertAt);
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
    (contextId: string) => {
      if (!onProjectIdsChange) return;
      const next = selectedProjectIds.filter(
        (existing) => existing !== contextId,
      );
      if (next.length === selectedProjectIds.length) return;
      onProjectIdsChange(next);
    },
    [onProjectIdsChange, selectedProjectIds],
  );

  const referencesItems = useMemo(
    () => selectedProjectContextItems(projectItems, selectedProjectIds),
    [projectItems, selectedProjectIds],
  );

  const dismissPendingInsert = useCallback(() => {
    setPendingInsert(null);
    setRangeWarning(null);
  }, []);

  return {
    editor,
    projectContextEnabled,
    referencesItems,
    onProjectIdsChange,
    removeSelectedProjectId,
    pendingInsert,
    rangeWarning,
    dismissPendingInsert,
    insertPathOnly,
    insertWithRange,
    pendingProjectChoice,
    projectEdges,
    selectedProjectIds,
    cancelProjectContextChoice,
    confirmProjectContextChoice,
    repoHints,
  };
}

function RichPromptFileReferenceModal({
  id,
  pendingInsert,
  disabled,
  rangeWarning,
  onClose,
  onInsertWithRange,
  onInsertPathOnly,
}: {
  id: string;
  pendingInsert: PendingFileInsert;
  disabled?: boolean;
  rangeWarning: string | null;
  onClose: () => void;
  onInsertWithRange: (startLine: number, endLine: number) => Promise<void>;
  onInsertPathOnly: () => void;
}) {
  const rangeModalTitleId = `${id}-mention-range-modal-title`;
  const rangeModalDescId = `${id}-mention-range-modal-desc`;

  return (
    <Modal
      onClose={onClose}
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
          onInsertWithRange={onInsertWithRange}
          onInsertPathOnly={onInsertPathOnly}
          onCancel={onClose}
        />
      </section>
    </Modal>
  );
}

/** Rich initial prompt (TipTap) with @ file suggestions when the workspace repo (app_settings.repo_root) is set, plus optional `#` project-context mentions. */
export function RichPromptEditor(props: Props) {
  const { id, disabled } = props;
  const {
    editor,
    projectContextEnabled,
    referencesItems,
    onProjectIdsChange,
    removeSelectedProjectId,
    pendingInsert,
    rangeWarning,
    dismissPendingInsert,
    insertPathOnly,
    insertWithRange,
    pendingProjectChoice,
    projectEdges,
    selectedProjectIds,
    cancelProjectContextChoice,
    confirmProjectContextChoice,
    repoHints,
  } = useRichPromptEditorController(props);

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
        <RichPromptFileReferenceModal
          id={id}
          pendingInsert={pendingInsert}
          disabled={disabled}
          rangeWarning={rangeWarning}
          onClose={dismissPendingInsert}
          onInsertWithRange={insertWithRange}
          onInsertPathOnly={insertPathOnly}
        />
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
        showRepoMisconfigHint={repoHints.showRepoMisconfigHint}
        workspaceBroken={repoHints.workspaceBroken}
        fileSearchFailedWhileAvailable={
          repoHints.fileSearchFailedWhileAvailable
        }
        showRepoUnknownHint={repoHints.showRepoUnknownHint}
        showFileSearchSpinner={repoHints.showFileSearchSpinner}
      />
    </div>
  );
}
