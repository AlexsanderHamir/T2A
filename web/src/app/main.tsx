import "tippy.js/dist/tippy.css";
import { QueryClientProvider } from "@tanstack/react-query";
import { PersistQueryClientProvider } from "@tanstack/react-query-persist-client";
import { createSyncStoragePersister } from "@tanstack/query-sync-storage-persister";
import React, { useState } from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "./App";
import { createAppQueryClient } from "../lib/queryClient";
import {
  isQueryPersistEnabled,
  queryPersistMaxAgeMs,
  queryPersistStorageKey,
  shouldPersistQuery,
} from "../lib/queryPersist";
import { ROUTER_FUTURE_FLAGS } from "../lib/routerFutureFlags";
import { AppErrorBoundary } from "../shared/AppErrorBoundary";
import { ToastProvider } from "../shared/toast";
import { installRUM } from "../observability";

const queryClient = createAppQueryClient();

installRUM();

function QueryProviders({ children }: { children: React.ReactNode }) {
  if (!isQueryPersistEnabled() || typeof window === "undefined") {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }

  const persister = createSyncStoragePersister({
    storage: window.sessionStorage,
    key: queryPersistStorageKey(),
  });

  return (
    <PersistQueryClientProvider
      client={queryClient}
      persistOptions={{
        persister,
        maxAge: queryPersistMaxAgeMs(),
        dehydrateOptions: {
          shouldDehydrateQuery: (query) => shouldPersistQuery(query),
        },
      }}
    >
      {children}
    </PersistQueryClientProvider>
  );
}

function AppWithRecoveryBoundary() {
  const [appKey, setAppKey] = useState(0);
  return (
    <AppErrorBoundary onRecover={() => setAppKey((k) => k + 1)}>
      <App key={appKey} />
    </AppErrorBoundary>
  );
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter future={ROUTER_FUTURE_FLAGS}>
      <QueryProviders>
        <ToastProvider>
          <AppWithRecoveryBoundary />
        </ToastProvider>
      </QueryProviders>
    </BrowserRouter>
  </React.StrictMode>,
);
