import { useRef } from "react";
import type { UseQueryResult } from "@tanstack/react-query";

/** Latch live-inventory failure so refetches during reconcile do not hide the prompt. */
export function useLiveInventoryUnreachable(
  query: Pick<UseQueryResult, "isError" | "isSuccess" | "isLoading">,
): boolean {
  const hadErrorRef = useRef(false);
  if (query.isSuccess) {
    hadErrorRef.current = false;
  }
  if (query.isError) {
    hadErrorRef.current = true;
  }
  return hadErrorRef.current && !query.isLoading;
}
