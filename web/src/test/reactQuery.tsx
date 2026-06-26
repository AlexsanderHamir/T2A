import type { AppSettings } from "@/api/settings";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { vi } from "vitest";
import { ToastProvider } from "@/shared/toast";
import { settingsQueryKeys } from "@/settings/settingsQueryKeys";
import { APP_SETTINGS_DEFAULTS } from "./settingsDefaults";

export function makeAppSettings(overrides: Partial<AppSettings> = {}): AppSettings {
  return {
    ...APP_SETTINGS_DEFAULTS,
    ...overrides,
  };
}

/** QueryClient + settings seed + toast wrapper for mutation hook tests. */
export function makeMutationTestWrapper(settings: AppSettings = makeAppSettings()) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  queryClient.setQueryData(settingsQueryKeys.app(), settings);
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <ToastProvider>{children}</ToastProvider>
      </QueryClientProvider>
    );
  }
  return { Wrapper, queryClient, invalidateSpy };
}
