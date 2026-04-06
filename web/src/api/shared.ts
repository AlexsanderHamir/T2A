/** Shared headers and error text for JSON APIs. */
export const jsonHeaders = {
  "Content-Type": "application/json",
  Accept: "application/json",
};

const defaultFetchTimeoutMs = 20_000;

/** Upper bound for error response bodies (abuse / buggy proxies); aligns with bounded handler bodies. */
export const maxErrorResponseBodyBytes = 64 * 1024;

async function readResponseTextLimited(
  res: Response,
  maxBytes: number,
): Promise<string> {
  if (!res.body) {
    return "";
  }
  const reader = res.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      if (!value?.byteLength) {
        continue;
      }
      const remaining = maxBytes - total;
      if (remaining <= 0) {
        await reader.cancel();
        break;
      }
      if (value.byteLength <= remaining) {
        chunks.push(value);
        total += value.byteLength;
      } else {
        chunks.push(value.subarray(0, remaining));
        total += remaining;
        await reader.cancel();
        break;
      }
    }
  } catch {
    /* ignore stream read errors; fall through with partial data */
  }
  const merged = new Uint8Array(total);
  let offset = 0;
  for (const c of chunks) {
    merged.set(c, offset);
    offset += c.byteLength;
  }
  return new TextDecoder("utf-8", { fatal: false }).decode(merged);
}

function timeoutSignal(
  ms: number,
): { signal: AbortSignal | undefined; cleanup: (() => void) | undefined } {
  const AT = AbortSignal as typeof AbortSignal & {
    timeout?: (timeoutMs: number) => AbortSignal;
  };
  if (typeof AT.timeout !== "function") {
    const timeoutController = new AbortController();
    const timer = setTimeout(() => {
      timeoutController.abort();
    }, ms);
    return {
      signal: timeoutController.signal,
      cleanup: () => clearTimeout(timer),
    };
  }
  return {
    signal: AT.timeout(ms),
    cleanup: undefined,
  };
}

function combineSignals(
  user: AbortSignal | null | undefined,
  timeout: AbortSignal | undefined,
): AbortSignal | undefined {
  if (!user) return timeout;
  if (!timeout) return user;
  const AT = AbortSignal as typeof AbortSignal & {
    any?: (signals: AbortSignal[]) => AbortSignal;
  };
  if (typeof AT.any === "function") {
    return AT.any([user, timeout]);
  }
  const combined = new AbortController();
  const abortCombined = () => {
    if (!combined.signal.aborted) {
      combined.abort();
    }
  };
  user.addEventListener("abort", abortCombined, { once: true });
  timeout.addEventListener("abort", abortCombined, { once: true });
  if (user.aborted || timeout.aborted) {
    abortCombined();
  }
  return combined.signal;
}

export async function fetchWithTimeout(
  input: RequestInfo | URL,
  init?: RequestInit,
  options?: { timeoutMs?: number },
): Promise<Response> {
  const timeoutMs = options?.timeoutMs ?? defaultFetchTimeoutMs;
  const { signal: timeout, cleanup } = timeoutSignal(timeoutMs);
  const signal = combineSignals(init?.signal, timeout);
  try {
    return await fetch(input, { ...init, ...(signal ? { signal } : {}) });
  } finally {
    cleanup?.();
  }
}

export async function readError(res: Response): Promise<string> {
  const t = res.body
    ? await readResponseTextLimited(res, maxErrorResponseBodyBytes)
    : await res.text();
  try {
    const j = JSON.parse(t) as { error?: string; request_id?: string };
    const msg = typeof j?.error === "string" && j.error.trim() ? j.error.trim() : "";
    const rid =
      typeof j?.request_id === "string" && j.request_id.trim() ? j.request_id.trim() : "";
    if (msg) {
      return rid ? `${msg} (request ${rid})` : msg;
    }
    if (rid) {
      return `Error (request ${rid})`;
    }
  } catch {
    /* plain text */
  }
  return t.trim() || res.statusText;
}
