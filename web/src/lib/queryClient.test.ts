import { describe, expect, it, vi } from "vitest";
import { createAppQueryClient } from "./queryClient";

describe("createAppQueryClient", () => {
  it("logs query cache errors in development", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    const client = createAppQueryClient();
    const err = new Error("query failed");

    const onError = (client.getQueryCache() as unknown as {
      config?: { onError?: (error: unknown) => void };
    }).config?.onError;
    onError?.(err);

    expect(spy).toHaveBeenCalledWith("[tasks query]", err);
    spy.mockRestore();
  });

  it("logs mutation cache errors in development", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    const client = createAppQueryClient();
    const err = new Error("mutation failed");

    const onError = (client.getMutationCache() as unknown as {
      config?: { onError?: (error: unknown) => void };
    }).config?.onError;
    onError?.(err);

    expect(spy).toHaveBeenCalledWith("[tasks mutation]", err);
    spy.mockRestore();
  });
});
