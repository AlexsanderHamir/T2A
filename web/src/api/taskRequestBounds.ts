/** Match pkgs/tasks/handler path and query abuse guards (see docs/api.md). */

export const maxTaskPathIDBytes = 128;
export const maxListAfterIDParamBytes = 128;
export const maxListIntQueryParamBytes = 32;
/** Match pkgs/tasks/handler maxTemplateInstantiateCountPerItem. */
export const maxTemplateInstantiateCountPerItem = 25;
/** Match pkgs/tasks/handler maxTemplateInstantiateTotalCreates. */
export const maxTemplateInstantiateTotalCreates = 100;
/** Seq in path or before_seq / after_seq query (maxTaskEventSeqParamBytes). */
export const maxTaskSeqPathOrQueryParamBytes = 32;

export function assertTaskPathId(id: string, label = "id"): string {
  const t = id.trim();
  if (t.length === 0) {
    throw new Error(`${label} is required`);
  }
  if (t.length > maxTaskPathIDBytes) {
    throw new Error(`${label} is too long`);
  }
  return t;
}

/** When the field is optional; empty/whitespace-only is rejected if present. */
export function assertOptionalTaskPathId(
  id: string | undefined,
  label: string,
): string | undefined {
  if (id === undefined) {
    return undefined;
  }
  return assertTaskPathId(id, label);
}

export function assertAfterId(afterId: string): string {
  const t = afterId.trim();
  if (t.length === 0) {
    throw new Error("after_id is required when provided");
  }
  if (t.length > maxListAfterIDParamBytes) {
    throw new Error("after_id is too long");
  }
  return t;
}

export function assertListIntQuery(
  name: string,
  n: number,
  min: number,
  max: number,
): string {
  if (!Number.isFinite(n) || !Number.isInteger(n)) {
    throw new Error(`${name} must be an integer`);
  }
  if (n < min || n > max) {
    throw new Error(`${name} must be between ${min} and ${max}`);
  }
  const s = String(n);
  if (s.length > maxListIntQueryParamBytes) {
    throw new Error(`${name} query value is too long`);
  }
  return s;
}

export function assertNonNegativeOffset(name: string, n: number): string {
  if (!Number.isFinite(n) || !Number.isInteger(n) || n < 0) {
    throw new Error(`${name} must be a non-negative integer`);
  }
  const s = String(n);
  if (s.length > maxListIntQueryParamBytes) {
    throw new Error(`${name} query value is too long`);
  }
  return s;
}

export function assertPositiveSeq(name: string, n: number): string {
  if (!Number.isFinite(n) || !Number.isInteger(n) || n < 1) {
    throw new Error(`${name} must be a positive integer`);
  }
  const s = String(n);
  if (s.length > maxTaskSeqPathOrQueryParamBytes) {
    throw new Error(`${name} is too large`);
  }
  return s;
}

export type TaskTemplateInstantiateItem = {
  template_id: string;
  count: number;
};

export function assertInstantiateTemplateItems(
  items: TaskTemplateInstantiateItem[],
): TaskTemplateInstantiateItem[] {
  if (items.length === 0) {
    throw new Error("at least one template item is required");
  }
  const seen = new Set<string>();
  let total = 0;
  const normalized: TaskTemplateInstantiateItem[] = [];
  for (const item of items) {
    const template_id = assertTaskPathId(item.template_id, "template id");
    if (seen.has(template_id)) {
      throw new Error(`duplicate template id ${template_id}`);
    }
    seen.add(template_id);
    if (
      !Number.isInteger(item.count) ||
      item.count < 1 ||
      item.count > maxTemplateInstantiateCountPerItem
    ) {
      throw new Error(`count must be integer 1..${maxTemplateInstantiateCountPerItem}`);
    }
    total += item.count;
    if (total > maxTemplateInstantiateTotalCreates) {
      throw new Error(`total creates must not exceed ${maxTemplateInstantiateTotalCreates}`);
    }
    normalized.push({ template_id, count: item.count });
  }
  return normalized;
}
