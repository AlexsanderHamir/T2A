import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import { BrowserRouter, MemoryRouter } from "react-router-dom";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import App from "@/app/App";
import { bootstrapUnavailable } from "@/test/handlers/bootstrap";
import { stubEventSource } from "@/test/browserMocks";
import { repoNotConfigured } from "@/test/handlers/repo";
import { appSettingsOk } from "@/test/handlers/settings";
import { taskStatsEmpty, tasksListEmpty } from "@/test/handlers/tasks";

export function appDefaultHandlers() {
  return [
    bootstrapUnavailable(),
    appSettingsOk(),
    tasksListEmpty(),
    taskStatsEmpty(),
    repoNotConfigured(),
  ];
}

export function setupAppTest() {
  stubEventSource();
  try {
    window.sessionStorage.removeItem("hamix_ui_test_mode");
  } catch {
    /* private mode */
  }
}

export function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

export function renderApp() {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter future={ROUTER_FUTURE_FLAGS}>
        <App />
      </BrowserRouter>
    </QueryClientProvider>,
  );
}

export function renderAppAt(initialEntries: string[]) {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={initialEntries}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}
