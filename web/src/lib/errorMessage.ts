/**
 * Coerce an unknown thrown value into a user-presentable string.
 *
 * Catch handlers across the app surface this through banners (`.err` /
 * `error-banner`) and React Query's error objects, which can be `Error`
 * instances, plain strings, server-shaped JSON wrapped in `Error`, or — for
 * legacy code paths — bare values. Centralizing the coercion keeps the wire
 * shape we expose to users consistent and avoids accidental "[object Object]"
 * banners when something throws a non-Error value.
 */
export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}
