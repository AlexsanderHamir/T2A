/**
 * Public surface for the toast feature. Hooks and pages should import
 * from `@/shared/toast` (or relative paths inside shared/) — never
 * deep-import the provider/file directly so a future move stays
 * confined to this barrel.
 */
export { ToastProvider, useToast, useOptionalToast } from "./ToastProvider";
export type { ToastKind, ToastItem } from "./ToastProvider";
