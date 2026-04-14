import { useState } from "react";
import { Outlet, useLocation } from "react-router-dom";
import { AppErrorBoundary } from "../shared/AppErrorBoundary";

export function RoutedMainOutlet() {
  const location = useLocation();
  const [outletKey, setOutletKey] = useState(0);
  return (
    <AppErrorBoundary
      key={location.pathname}
      variant="route-outlet"
      fallbackMessage="Something went wrong in this view."
      onRecover={() => setOutletKey((k) => k + 1)}
    >
      <Outlet key={outletKey} />
    </AppErrorBoundary>
  );
}
