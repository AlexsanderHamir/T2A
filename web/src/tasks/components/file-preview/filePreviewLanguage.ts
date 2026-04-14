export type FilePreviewLanguage = {
  label: string;
  prism: string;
};

const FALLBACK: FilePreviewLanguage = { label: "Plain text", prism: "plain" };

const LANGUAGE_BY_EXTENSION: Record<string, FilePreviewLanguage> = {
  c: { label: "C", prism: "c" },
  h: { label: "C Header", prism: "c" },
  cc: { label: "C++", prism: "cpp" },
  cpp: { label: "C++", prism: "cpp" },
  cxx: { label: "C++", prism: "cpp" },
  hpp: { label: "C++ Header", prism: "cpp" },
  cs: { label: "C#", prism: "csharp" },
  go: { label: "Go", prism: "go" },
  ts: { label: "TypeScript", prism: "typescript" },
  tsx: { label: "TSX", prism: "tsx" },
  js: { label: "JavaScript", prism: "javascript" },
  jsx: { label: "JSX", prism: "jsx" },
  json: { label: "JSON", prism: "json" },
  yml: { label: "YAML", prism: "yaml" },
  yaml: { label: "YAML", prism: "yaml" },
  md: { label: "Markdown", prism: "markdown" },
  py: { label: "Python", prism: "python" },
  sh: { label: "Shell", prism: "bash" },
  bash: { label: "Shell", prism: "bash" },
  zsh: { label: "Shell", prism: "bash" },
  fish: { label: "Shell", prism: "bash" },
  sql: { label: "SQL", prism: "sql" },
  css: { label: "CSS", prism: "css" },
  html: { label: "HTML", prism: "markup" },
  xml: { label: "XML", prism: "markup" },
  java: { label: "Java", prism: "java" },
  rb: { label: "Ruby", prism: "ruby" },
  rs: { label: "Rust", prism: "rust" },
  toml: { label: "TOML", prism: "toml" },
  ini: { label: "INI", prism: "ini" },
  conf: { label: "Config", prism: "ini" },
  diff: { label: "Diff", prism: "diff" },
  patch: { label: "Patch", prism: "diff" },
};

/** Match server `maxRepoRelPathQueryBytes`; only the tail affects basename/extension. */
const maxPathCharsForLanguageDetection = 4096;

export function filePreviewLanguageFromPath(path: string): FilePreviewLanguage {
  const trimmed = path.trim();
  if (trimmed === "") {
    return FALLBACK;
  }
  const tail =
    trimmed.length > maxPathCharsForLanguageDetection
      ? trimmed.slice(-maxPathCharsForLanguageDetection)
      : trimmed;
  const lastSlash = tail.lastIndexOf("/");
  const base = lastSlash >= 0 ? tail.slice(lastSlash + 1) : tail;
  const lowerBase = base.toLowerCase();
  if (lowerBase === "dockerfile") {
    return { label: "Dockerfile", prism: "docker" };
  }
  if (lowerBase === ".gitignore" || lowerBase === ".gitattributes") {
    return { label: "Git", prism: "git" };
  }
  const lastDot = base.lastIndexOf(".");
  if (lastDot <= 0 || lastDot >= base.length - 1) {
    return FALLBACK;
  }
  const ext = base.slice(lastDot + 1).toLowerCase();
  if (!ext) return FALLBACK;
  return LANGUAGE_BY_EXTENSION[ext] ?? FALLBACK;
}
