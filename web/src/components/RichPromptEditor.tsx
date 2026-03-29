import { EditorContent, useEditor } from "@tiptap/react";
import type { Editor } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  RepoFileSuggestion,
  type RepoFileSuggestionOptions,
} from "../extensions/repoFileSuggestion";
import { validateRepoRange } from "../api";
import {
  looksLikeStoredHtml,
  plainTextToInitialHtml,
} from "../promptFormat";

type Props = {
  id: string;
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  placeholder?: string;
};

function MenuBar({
  editor,
  disabled,
}: {
  editor: Editor | null;
  disabled?: boolean;
}) {
  if (!editor) return null;
  const d = Boolean(disabled);
  return (
    <div
      className="rich-prompt-toolbar"
      role="toolbar"
      aria-label="Text formatting"
    >
      <button
        type="button"
        className="secondary toolbar-btn"
        disabled={d || !editor.can().chain().focus().toggleBold().run()}
        onClick={() => editor.chain().focus().toggleBold().run()}
        aria-pressed={editor.isActive("bold")}
      >
        Bold
      </button>
      <button
        type="button"
        className="secondary toolbar-btn"
        disabled={d || !editor.can().chain().focus().toggleItalic().run()}
        onClick={() => editor.chain().focus().toggleItalic().run()}
        aria-pressed={editor.isActive("italic")}
      >
        Italic
      </button>
      <button
        type="button"
        className="secondary toolbar-btn"
        disabled={d}
        onClick={() =>
          editor.chain().focus().toggleHeading({ level: 2 }).run()
        }
        aria-pressed={editor.isActive("heading", { level: 2 })}
      >
        H2
      </button>
      <button
        type="button"
        className="secondary toolbar-btn"
        disabled={d}
        onClick={() =>
          editor.chain().focus().toggleHeading({ level: 3 }).run()
        }
        aria-pressed={editor.isActive("heading", { level: 3 })}
      >
        H3
      </button>
      <button
        type="button"
        className="secondary toolbar-btn"
        disabled={d || !editor.can().chain().focus().toggleBulletList().run()}
        onClick={() => editor.chain().focus().toggleBulletList().run()}
        aria-pressed={editor.isActive("bulletList")}
      >
        List
      </button>
      <button
        type="button"
        className="secondary toolbar-btn"
        disabled={d || !editor.can().chain().focus().toggleOrderedList().run()}
        onClick={() => editor.chain().focus().toggleOrderedList().run()}
        aria-pressed={editor.isActive("orderedList")}
      >
        Numbered
      </button>
      <button
        type="button"
        className="secondary toolbar-btn"
        disabled={d || !editor.can().chain().focus().toggleCode().run()}
        onClick={() => editor.chain().focus().toggleCode().run()}
        aria-pressed={editor.isActive("code")}
      >
        Code
      </button>
      <button
        type="button"
        className="secondary toolbar-btn"
        disabled={d}
        onClick={() => editor.chain().focus().setParagraph().run()}
      >
        Paragraph
      </button>
    </div>
  );
}

/** Rich initial prompt (TipTap) with @ file suggestions when REPO_ROOT is set. */
export function RichPromptEditor({
  id,
  value,
  onChange,
  disabled,
  placeholder,
}: Props) {
  const [repoUnavailable, setRepoUnavailable] = useState(false);
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
      onRepoUnavailable: () => setRepoUnavailable(true),
      onRepoAvailable: () => setRepoUnavailable(false),
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

  const insertPathOnly = () => {
    if (!editor || !pendingInsert) return;
    const { insertAt, path } = pendingInsert;
    const token = `@${path} `;
    editor.chain().focus().insertContentAt(insertAt, token).run();
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
        .insertContentAt(insertAt, `@${path}(${a}-${b}) `)
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
      .insertContentAt(insertAt, `@${path}(${a}-${b}) `)
      .run();
    setPendingInsert(null);
  };

  return (
    <div className="rich-prompt-wrap">
      <MenuBar editor={editor} disabled={disabled} />
      <EditorContent editor={editor} />
      {pendingInsert ? (
        <div
          className="mention-range-panel"
          role="region"
          aria-label="Optional line range for file mention"
        >
          <p className="muted stack-tight-zero">
            <code>{pendingInsert.path}</code>
          </p>
          <p className="muted stack-tight-zero mention-range-hint">
            Add a line range (optional), or insert the file reference only.
          </p>
          <div className="row mention-range-row">
            <div className="field">
              <label htmlFor={`${id}-line-start`}>From line</label>
              <input
                id={`${id}-line-start`}
                type="number"
                min={1}
                value={lineStart}
                disabled={disabled}
                onChange={(e) => setLineStart(e.target.value)}
              />
            </div>
            <div className="field">
              <label htmlFor={`${id}-line-end`}>To line</label>
              <input
                id={`${id}-line-end`}
                type="number"
                min={1}
                value={lineEnd}
                disabled={disabled}
                onChange={(e) => setLineEnd(e.target.value)}
              />
            </div>
          </div>
          {rangeWarning ? (
            <p className="mention-warn" role="alert">
              {rangeWarning}
            </p>
          ) : null}
          <div className="row stack-row-actions">
            <button
              type="button"
              disabled={disabled}
              onClick={() => void insertWithRange()}
            >
              Insert with range
            </button>
            <button
              type="button"
              className="secondary"
              disabled={disabled}
              onClick={insertPathOnly}
            >
              Insert file only
            </button>
            <button
              type="button"
              className="secondary"
              disabled={disabled}
              onClick={() => {
                setPendingInsert(null);
                setRangeWarning(null);
              }}
            >
              Cancel
            </button>
          </div>
        </div>
      ) : null}
      {repoUnavailable ? (
        <p className="mention-repo-hint" role="status">
          No repository is configured for file search. Set{" "}
          <code>REPO_ROOT</code> in the server environment (same{" "}
          <code>.env</code> as <code>DATABASE_URL</code>) and restart{" "}
          <code>taskapi</code>.
        </p>
      ) : null}
    </div>
  );
}
