import { EditorContent, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { RepoFileMention } from "../../extensions/repoFileMention";
import {
  RepoFileSuggestion,
  type RepoFileSuggestionOptions,
} from "../../extensions/repoFileSuggestion";
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

type Props = {
  id: string;
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  placeholder?: string;
};

/** Rich initial prompt (TipTap) with @ file suggestions when the workspace repo (app_settings.repo_root) is set. */
export function RichPromptEditor({
  id,
  value,
  onChange,
  disabled,
  placeholder,
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
    ],
    [placeholder, repoOpts],
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

  return (
    <div className="rich-prompt-wrap">
      <RichPromptMenuBar editor={editor} disabled={disabled} />
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
