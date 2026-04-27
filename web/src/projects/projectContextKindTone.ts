import type { ProjectContextKind } from "@/types";

export function projectContextKindTone(kind: ProjectContextKind) {
  const normalized = kind.trim().toLowerCase();
  let hash = 0;
  for (const char of normalized) {
    hash = (hash * 31 + char.charCodeAt(0)) % 997;
  }
  return String(hash % 5);
}
