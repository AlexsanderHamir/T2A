export const taskQueryKeys = {
  all: ["tasks"] as const,
  list: () => [...taskQueryKeys.all, "list"] as const,
};
