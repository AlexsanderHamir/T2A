import { interceptUiTestModeFetch } from "@/dev/uiTestModeInterceptor";

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
  const synthetic = interceptUiTestModeFetch(input, init);
  if (synthetic) return synthetic;
  const timeoutMs = options?.timeoutMs ?? defaultFetchTimeoutMs;
  const { signal: timeout, cleanup } = timeoutSignal(timeoutMs);
  const signal = combineSignals(init?.signal, timeout);
  try {
    return await fetch(input, { ...init, ...(signal ? { signal } : {}) });
  } finally {
    cleanup?.();
  }
}

/**
 * Typed error thrown by API helpers when a response is not ok.
 *
 * Carries the HTTP status, the server-provided `error` message, an
 * optional machine-readable `code` and the `request_id` (when the
 * backend includes one) so call sites can branch on status without
 * regex-matching the message:
 *
 *     try { await getTask(id); }
 *     catch (err) {
 *       if (err instanceof ApiError && err.status === 404) {
 *         // Show "not found" empty state.
 *       }
 *       throw err;
 *     }
 *
 * `ApiError extends Error`, so all existing `instanceof Error` checks
 * and React Query error surfaces keep working without changes.
 */
export class ApiError extends Error {
  readonly status: number;
  readonly code?: string;
  readonly requestId?: string;

  constructor(
    message: string,
    init: { status: number; code?: string; requestId?: string },
  ) {
    super(message);
    this.name = "ApiError";
    this.status = init.status;
    this.code = init.code;
    this.requestId = init.requestId;
  }
}

type ParsedErrorBody = {
  message: string;
  code?: string;
  requestId?: string;
};

async function parseErrorBody(res: Response): Promise<ParsedErrorBody> {
  const t = res.body
    ? await readResponseTextLimited(res, maxErrorResponseBodyBytes)
    : await res.text();
  try {
    const j = JSON.parse(t) as {
      error?: string;
      code?: string;
      request_id?: string;
    };
    const msg =
      typeof j?.error === "string" && j.error.trim() ? j.error.trim() : "";
    const code =
      typeof j?.code === "string" && j.code.trim() ? j.code.trim() : undefined;
    const rid =
      typeof j?.request_id === "string" && j.request_id.trim()
        ? j.request_id.trim()
        : undefined;
    if (msg) {
      return { message: msg, code, requestId: rid };
    }
    if (rid) {
      return { message: "Error", code, requestId: rid };
    }
  } catch {
    /* plain text */
  }
  return { message: t.trim() || res.statusText };
}

export async function readError(res: Response): Promise<string> {
  const { message, requestId } = await parseErrorBody(res);
  return requestId ? `${message} (request ${requestId})` : message;
}

/**
 * Build a typed `ApiError` from a non-ok `Response`. The legacy string
 * form (with the request id appended in parentheses) is preserved as
 * `.message` so existing UI that renders `error.message` keeps the
 * same output.
 */
export async function apiErrorFromResponse(res: Response): Promise<ApiError> {
  const { message, code, requestId } = await parseErrorBody(res);
  const display = requestId ? `${message} (request ${requestId})` : message;
  return new ApiError(display, {
    status: res.status,
    code,
    requestId,
  });
}
