import "tippy.js/dist/tippy.css";
import { QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "./App";
import { createAppQueryClient } from "../lib/queryClient";
import { ROUTER_FUTURE_FLAGS } from "../lib/routerFutureFlags";

const queryClient = createAppQueryClient();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter future={ROUTER_FUTURE_FLAGS}>
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>
    </BrowserRouter>
  </React.StrictMode>,
);
