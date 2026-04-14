export function focusableSelector(): string {
  return [
    "a[href]:not([disabled])",
    "button:not([disabled])",
    "textarea:not([disabled])",
    'input:not([disabled]):not([type="hidden"])',
    "select:not([disabled])",
    '[tabindex]:not([tabindex="-1"])',
  ].join(", ");
}

export function listFocusables(shell: HTMLElement): HTMLElement[] {
  return Array.from(shell.querySelectorAll<HTMLElement>(focusableSelector())).filter(
    (el) => !el.closest("[aria-hidden='true']"),
  );
}
