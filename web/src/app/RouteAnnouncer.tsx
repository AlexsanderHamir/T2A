import { useEffect, useState } from "react";
import { useLocation } from "react-router-dom";

/**
 * Announces the active view to assistive tech on client-side navigation. Page
 * components set {@link document.title} in effects; this runs after them (sibling
 * under `.app` is shallower than `main`’s route subtree) and mirrors that title
 * into an aria-live region (WCAG 4.1.3).
 */
export function RouteAnnouncer() {
  const location = useLocation();
  const [message, setMessage] = useState(() => document.title);

  useEffect(() => {
    setMessage(document.title);
  }, [location.pathname, location.search, location.key]);

  return (
    <div
      className="route-announcer visually-hidden"
      aria-live="polite"
      aria-atomic="true"
    >
      {message}
    </div>
  );
}
