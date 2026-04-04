/** SSE `data:` payloads are JSON lines `{ "type": "...", "id": "<uuid>" }` (see docs/DESIGN.md). */
export function collectTaskIdFromSSEData(data: string, into: Set<string>): void {
  const trimmed = data.trim();
  if (trimmed === "") {
    return;
  }
  try {
    const o = JSON.parse(trimmed) as { id?: unknown };
    if (typeof o.id !== "string") {
      return;
    }
    const id = o.id.trim();
    if (id !== "") {
      into.add(id);
    }
  } catch {
    // Ignore non-JSON frames; caller may fall back to a broad invalidation.
  }
}
