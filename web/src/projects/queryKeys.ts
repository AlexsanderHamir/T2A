export const projectQueryKeys = {
  all: ["projects"] as const,
  list: (includeArchived = false, limit = 50) =>
    [...projectQueryKeys.all, "list", includeArchived, limit] as const,
  detail: (id: string) => [...projectQueryKeys.all, "detail", id] as const,
  context: (id: string) =>
    [...projectQueryKeys.all, "detail", id, "context"] as const,
};
