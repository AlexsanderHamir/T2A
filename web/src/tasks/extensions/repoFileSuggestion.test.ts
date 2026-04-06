import { Editor } from "@tiptap/core";
import Placeholder from "@tiptap/extension-placeholder";
import StarterKit from "@tiptap/starter-kit";
import { waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RepoFileMention } from "./repoFileMention";
import { RepoFileSuggestion } from "./repoFileSuggestion";

describe("RepoFileSuggestion", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ paths: ["a/b.go"] }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("invokes onRepoUnavailable when /repo/search returns 503", async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(null, { status: 503 }));
    const onRepoUnavailable = vi.fn();
    const editor = new Editor({
      extensions: [
        StarterKit,
        Placeholder.configure({ placeholder: "" }),
        RepoFileMention,
        RepoFileSuggestion.configure({ onRepoUnavailable }),
      ],
      content: "<p></p>",
    });
    editor.chain().insertContent("@").run();
    await waitFor(() => expect(onRepoUnavailable).toHaveBeenCalled());
    editor.destroy();
  });

  it("does not invoke onRepoUnavailable when fetch throws (transient error)", async () => {
    vi.mocked(fetch).mockRejectedValueOnce(new Error("network"));
    const onRepoUnavailable = vi.fn();
    const editor = new Editor({
      extensions: [
        StarterKit,
        Placeholder.configure({ placeholder: "" }),
        RepoFileMention,
        RepoFileSuggestion.configure({ onRepoUnavailable }),
      ],
      content: "<p></p>",
    });
    editor.chain().insertContent("@").run();
    await waitFor(() => expect(vi.mocked(fetch)).toHaveBeenCalled());
    await new Promise((r) => setTimeout(r, 30));
    expect(onRepoUnavailable).not.toHaveBeenCalled();
    editor.destroy();
  });

  it("invokes onRepoAvailable when search succeeds", async () => {
    const onRepoAvailable = vi.fn();
    const editor = new Editor({
      extensions: [
        StarterKit,
        Placeholder.configure({ placeholder: "" }),
        RepoFileMention,
        RepoFileSuggestion.configure({ onRepoAvailable }),
      ],
      content: "<p></p>",
    });
    editor.chain().insertContent("@").run();
    await waitFor(() => expect(onRepoAvailable).toHaveBeenCalled());
    editor.destroy();
  });
});
