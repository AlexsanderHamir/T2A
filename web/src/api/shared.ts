/** Shared headers and error text for JSON APIs. */
export const jsonHeaders = {
  "Content-Type": "application/json",
  Accept: "application/json",
};

export async function readError(res: Response): Promise<string> {
  const t = await res.text();
  try {
    const j = JSON.parse(t) as { error?: string };
    if (typeof j?.error === "string" && j.error.trim()) {
      return j.error.trim();
    }
  } catch {
    /* plain text */
  }
  return t.trim() || res.statusText;
}
