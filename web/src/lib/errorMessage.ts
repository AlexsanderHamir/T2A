/**
 * Coerce an unknown thrown value into a user-presentable string.
 *
 * Catch handlers across the app surface this through banners (`.err` /
 * `error-banner`) and React Query's error objects, which can be `Error`
 * instances, plain strings, server-shaped JSON wrapped in `Error`, or — for
 * legacy code paths — bare values. Centralizing the coercion keeps the wire
 * shape we expose to users consistent and avoids accidental "[object Object]"
 * banners when something throws a non-Error value.
 *
 * Pass `fallback` when the surrounding UI has a kinder default than `String(e)`
 * (e.g. "Could not load updates."). The fallback is only used for non-Error
 * inputs; an `Error` always wins so the original message reaches the banner.
 */
export function errorMessage(e: unknown, fallback?: string): string {
  if (e instanceof Error) {
    return e.message;
  }
  return fallback ?? String(e);
}
