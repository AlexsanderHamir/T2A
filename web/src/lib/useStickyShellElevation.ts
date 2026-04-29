import { useEffect, useState } from "react";

/**
 * Tracks whether the page has been scrolled past `threshold` pixels from
 * the top, so a sticky header can lift to a small elevation once it
 * detaches from the page edge.
 *
 * Returns a stable boolean — use it as a `data-elevated` attribute on
 * the sticky surface and let CSS handle the actual shadow/border
 * transition. Matches the Stripe / Linear dashboard top-nav scroll
 * behavior where the chrome reads as "floating" once the operator has
 * scrolled away from the page top.
 *
 * SSR-safe: returns `false` during the first render on the server (no
 * `window`), then settles on the real value after the first effect on
 * the client. The initial-paint flash is avoided by reading
 * `window.scrollY` synchronously inside the effect *before* binding the
 * scroll listener, so scroll-restored pages start in the correct
 * elevated state.
 *
 * Listener is `{ passive: true }` so it never stalls scroll on mobile.
 */
export function useStickyShellElevation(threshold: number = 4): boolean {
  const [elevated, setElevated] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined") return;

    const compute = () => {
      // `scrollY` reads as a number on every modern browser; the
      // `Math.max` with 0 guards against negative values that some
      // platforms surface during over-scroll bounce (iOS rubber-band).
      setElevated(Math.max(0, window.scrollY) > threshold);
    };

    compute();
    window.addEventListener("scroll", compute, { passive: true });
    return () => {
      window.removeEventListener("scroll", compute);
    };
  }, [threshold]);

  return elevated;
}
