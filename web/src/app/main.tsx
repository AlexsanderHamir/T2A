import "tippy.js/dist/tippy.css";
import { QueryClientProvider } from "@tanstack/react-query";
import React, { useState } from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "./App";
import { createAppQueryClient } from "../lib/queryClient";
import { ROUTER_FUTURE_FLAGS } from "../lib/routerFutureFlags";
import { AppErrorBoundary } from "../shared/AppErrorBoundary";
import { installRUM } from "../observability";

const queryClient = createAppQueryClient();

// Install the RUM beacon once at module load. Idempotent under
// React.StrictMode and tolerant of missing window/document so SSR or
// jsdom test imports don't blow up.
installRUM();

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
      <QueryClientProvider client={queryClient}>
        <AppWithRecoveryBoundary />
      </QueryClientProvider>
    </BrowserRouter>
  </React.StrictMode>,
);
