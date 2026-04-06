import { EditorContent, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { RepoFileMention } from "../extensions/repoFileMention";
import {
  RepoFileSuggestion,
  type RepoFileSuggestionOptions,
} from "../extensions/repoFileSuggestion";
import {
  probeRepoWorkspace,
  validateRepoRange,
  type RepoWorkspaceProbe,
} from "../../api";
import {
  looksLikeStoredHtml,
  plainTextToInitialHtml,
} from "../promptFormat";
import { MentionRangePanel } from "./MentionRangePanel";
import { RichPromptMenuBar } from "./RichPromptMenuBar";

type Props = {
  id: string;
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  placeholder?: string;
};

/** Rich initial prompt (TipTap) with @ file suggestions when REPO_ROOT is set. */
export function RichPromptEditor({
  id,
  value,
  onChange,
  disabled,
  placeholder,
}: Props) {
  const [workspaceProbe, setWorkspaceProbe] = useState<
    RepoWorkspaceProbe | "pending"
  >("pending");
  const [fileSearchUnavailable, setFileSearchUnavailable] = useState(false);
  const [fileSuggestBusy, setFileSuggestBusy] = useState(false);
  const [pendingInsert, setPendingInsert] = useState<{
    insertAt: number;
    path: string;
  } | null>(null);
  const [lineStart, setLineStart] = useState("");
  const [lineEnd, setLineEnd] = useState("");
  const [rangeWarning, setRangeWarning] = useState<string | null>(null);
  const lastEmittedHtml = useRef<string | null>(null);

  const onFilePicked = useCallback(
    (payload: { insertAt: number; path: string }) => {
      setPendingInsert({ insertAt: payload.insertAt, path: payload.path });
      setLineStart("");
      setLineEnd("");
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
    let cancelled = false;
    setWorkspaceProbe("pending");
    void probeRepoWorkspace().then((p) => {
      if (!cancelled) setWorkspaceProbe(p);
    });
    return () => {
      cancelled = true;
    };
  }, []);

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

  const showFileSearchSpinner =
    probeDone &&
    workspaceProbe.state === "available" &&
    fileSuggestBusy &&
    !fileSearchUnavailable;

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

  const insertWithRange = async () => {
    if (!editor || !pendingInsert) return;
    const { insertAt, path } = pendingInsert;
    const a = parseInt(lineStart, 10);
    const b = parseInt(lineEnd, 10);
    if (!Number.isFinite(a) || !Number.isFinite(b)) {
      setRangeWarning("Enter start and end line numbers.");
      return;
    }
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
        <MentionRangePanel
          id={id}
          path={pendingInsert.path}
          disabled={disabled}
          lineStart={lineStart}
          lineEnd={lineEnd}
          rangeWarning={rangeWarning}
          onLineStartChange={setLineStart}
          onLineEndChange={setLineEnd}
          onInsertWithRange={insertWithRange}
          onInsertPathOnly={insertPathOnly}
          onCancel={() => {
            setPendingInsert(null);
            setRangeWarning(null);
          }}
        />
      ) : null}
      {showRepoMisconfigHint ? (
        <p className="mention-repo-hint" role="status">
          {workspaceProbe.state === "broken" ? (
            <>
              The workspace folder for <code>REPO_ROOT</code> is missing or not a
              directory on the machine running <code>taskapi</code>. Fix the path
              and restart <code>taskapi</code>.
            </>
          ) : workspaceProbe.state === "available" && fileSearchUnavailable ? (
            <>
              File search failed even though the server reports a workspace.
              Restart <code>taskapi</code> or check server logs.
            </>
          ) : (
            <>
              No repository is configured for file search. Set{" "}
              <code>REPO_ROOT</code> in the server environment (same{" "}
              <code>.env</code> as <code>DATABASE_URL</code>) and restart{" "}
              <code>taskapi</code> from the repo root so it loads that{" "}
              <code>.env</code>.
            </>
          )}
        </p>
      ) : null}
      {showRepoUnknownHint ? (
        <p className="mention-repo-hint" role="status">
          Could not verify workspace file search. For local dev, run{" "}
          <code>taskapi</code> and the Vite dev server so <code>/health</code>{" "}
          and <code>/repo</code> proxy to the API (see <code>web/vite.config</code>
          ).
        </p>
      ) : null}
      {showFileSearchSpinner ? (
        <p
          className="mention-repo-hint mention-repo-hint--pending"
          role="status"
          aria-live="polite"
        >
          Searching files…
        </p>
      ) : null}
    </div>
  );
}
