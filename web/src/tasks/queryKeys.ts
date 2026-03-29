export const taskQueryKeys = {
  all: ["tasks"] as const,
  list: () => [...taskQueryKeys.all, "list"] as const,
  detail: (id: string) => [...taskQueryKeys.all, "detail", id] as const,
  events: (id: string) => [...taskQueryKeys.all, "detail", id, "events"] as const,
};
