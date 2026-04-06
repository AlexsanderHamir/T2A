/**
 * Maps a textarea selection (start/end exclusive like the DOM) to 1-based inclusive line numbers.
 * Returns null when the selection is empty.
 */
export function lineRangeFromSelection(
  text: string,
  selStart: number,
  selEnd: number,
): { startLine: number; endLine: number } | null {
  if (selStart === selEnd) return null;
  const a = Math.min(selStart, selEnd);
  const b = Math.max(selStart, selEnd);
  const startLine = 1 + countNewlines(text.slice(0, a));
  const lastChar = Math.max(a, b - 1);
  const endLine = 1 + countNewlines(text.slice(0, lastChar + 1));
  return { startLine, endLine };
}

function countNewlines(s: string): number {
  let n = 0;
  for (let i = 0; i < s.length; i++) {
    if (s[i] === "\n") n++;
  }
  return n;
}
