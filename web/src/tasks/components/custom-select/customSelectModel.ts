export type CustomSelectOption =
  | { type: "header"; label: string }
  | {
      value: string;
      label: string;
      pillClass?: string;
      /** Visual indent steps for hierarchical lists (e.g. parent task picker). */
      depth?: number;
      /** Short leading label (e.g. Top level / Subtask in parent picker). */
      rowTag?: string;
    };

export function isCustomSelectHeader(
  o: CustomSelectOption,
): o is { type: "header"; label: string } {
  return "type" in o && o.type === "header";
}

export function firstSelectableIndex(opts: CustomSelectOption[]): number {
  const i = opts.findIndex((o) => !isCustomSelectHeader(o));
  return i >= 0 ? i : 0;
}

export function lastSelectableIndex(opts: CustomSelectOption[]): number {
  for (let i = opts.length - 1; i >= 0; i--) {
    if (!isCustomSelectHeader(opts[i])) return i;
  }
  return 0;
}

export function nextSelectable(
  opts: CustomSelectOption[],
  from: number,
): number {
  for (let i = from + 1; i < opts.length; i++) {
    if (!isCustomSelectHeader(opts[i])) return i;
  }
  for (let i = 0; i < opts.length; i++) {
    if (!isCustomSelectHeader(opts[i])) return i;
  }
  return from;
}

export function prevSelectable(
  opts: CustomSelectOption[],
  from: number,
): number {
  for (let i = from - 1; i >= 0; i--) {
    if (!isCustomSelectHeader(opts[i])) return i;
  }
  for (let i = opts.length - 1; i >= 0; i--) {
    if (!isCustomSelectHeader(opts[i])) return i;
  }
  return from;
}
