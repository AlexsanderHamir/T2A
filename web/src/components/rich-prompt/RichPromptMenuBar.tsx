import type { Editor } from "@tiptap/core";

export type RichPromptMenuBarProps = {
  editor: Editor | null;
  disabled?: boolean;
};

export function RichPromptMenuBar({ editor, disabled }: RichPromptMenuBarProps) {
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
