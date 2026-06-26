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

  it("treats whitespace-only path as plain text", () => {
    expect(filePreviewLanguageFromPath("   \t")).toEqual({
      label: "Plain text",
      prism: "plain",
    });
  });

  it("detects language from basename in very long paths without splitting the full string", () => {
    const suffix = "pkgs/tasks/handler/idempotency.go";
    const long = `${"x/".repeat(5000)}${suffix}`;
    expect(filePreviewLanguageFromPath(long)).toEqual({
      label: "Go",
      prism: "go",
    });
  });
});
