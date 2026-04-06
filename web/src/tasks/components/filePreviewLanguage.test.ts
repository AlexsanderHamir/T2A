import { describe, expect, it } from "vitest";
import { filePreviewLanguageFromPath } from "./filePreviewLanguage";

describe("filePreviewLanguageFromPath", () => {
  it("detects known extensions", () => {
    expect(filePreviewLanguageFromPath("pkgs/tasks/handler/idempotency.go")).toEqual(
      {
        label: "Go",
        prism: "go",
      },
    );
    expect(filePreviewLanguageFromPath("web/src/tasks/components/X.tsx")).toEqual({
      label: "TSX",
      prism: "tsx",
    });
    expect(filePreviewLanguageFromPath("docs/README.md")).toEqual({
      label: "Markdown",
      prism: "markdown",
    });
    expect(filePreviewLanguageFromPath("infra/Dockerfile")).toEqual({
      label: "Dockerfile",
      prism: "docker",
    });
    expect(filePreviewLanguageFromPath("native/main.cpp")).toEqual({
      label: "C++",
      prism: "cpp",
    });
  });

  it("falls back to plain text when unknown", () => {
    expect(filePreviewLanguageFromPath("foo/bar.unknown")).toEqual({
      label: "Plain text",
      prism: "plain",
    });
    expect(filePreviewLanguageFromPath("Makefile")).toEqual({
      label: "Plain text",
      prism: "plain",
    });
  });
});
