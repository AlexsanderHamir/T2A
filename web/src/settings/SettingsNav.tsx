import { useEffect, useRef, useState } from "react";

/**
 * Sticky in-page section index for the Settings page.
 *
 * The settings form spans ~2400px of vertical scroll across seven
 * sections. Without a navigation surface the operator's only path
 * to a specific knob is "scroll and scan." This component renders
 * a sticky list of section anchors and uses IntersectionObserver
 * to track which section is currently in view, mirroring the
 * pattern used by Stripe / Vercel / Linear settings pages.
 *
 * Visibility is desktop-only: at <1024px the page falls back to
 * vertical scroll without a nav rail (see the matching
 * @media rule in settings.css). The nav element is always
 * rendered so the IntersectionObserver wiring stays consistent
 * across breakpoints — only its container collapses.
 */
export type SettingsNavItem = {
  /** DOM id of the section element. */
  id: string;
  /** Sentence-case label rendered in the rail. */
  label: string;
};

export function SettingsNav({ items }: { items: SettingsNavItem[] }) {
  const [activeId, setActiveId] = useState<string>(items[0]?.id ?? "");
  // Track the most recently observed visible section. The
  // IntersectionObserver fires once per crossing, so we keep the
  // last-known id and only update React state when it changes —
  // avoids re-rendering the nav on every scroll tick.
  const lastActiveRef = useRef<string>(activeId);

  useEffect(() => {
    if (typeof IntersectionObserver === "undefined") return;
    const targets = items
      .map((it) => document.getElementById(it.id))
      .filter((el): el is HTMLElement => el !== null);
    if (targets.length === 0) return;

    const observer = new IntersectionObserver(
      (entries) => {
        // Pick the entry closest to the top of the viewport that's
        // currently intersecting. rootMargin biases the active band
        // to the upper third so the highlight matches what the
        // operator's eye is drawn to, not what's barely peeking in
        // at the bottom.
        const visible = entries
          .filter((e) => e.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible.length === 0) return;
        const id = visible[0].target.id;
        if (id !== lastActiveRef.current) {
          lastActiveRef.current = id;
          setActiveId(id);
        }
      },
      {
        // Top inset matches the sticky app header (~56px) plus a
        // small breathing band; bottom inset keeps the active
        // section locked in until the next section's heading
        // crosses ~30% of the viewport.
        rootMargin: "-80px 0px -55% 0px",
        threshold: [0, 0.25, 0.5, 1],
      },
    );
    targets.forEach((el) => observer.observe(el));
    return () => observer.disconnect();
  }, [items]);

  function handleClick(e: React.MouseEvent<HTMLAnchorElement>, id: string) {
    e.preventDefault();
    const el = document.getElementById(id);
    if (!el) return;
    const prefersReduced =
      typeof window.matchMedia === "function" &&
      window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    el.scrollIntoView({
      behavior: prefersReduced ? "auto" : "smooth",
      block: "start",
    });
    // Optimistically update the active marker so the click feels
    // immediate; the IntersectionObserver will reconcile within a
    // frame anyway.
    setActiveId(id);
    lastActiveRef.current = id;
  }

  return (
    <nav
      className="settings-nav"
      aria-label="Settings sections"
      data-testid="settings-nav"
    >
      <p className="settings-nav-label" aria-hidden="true">
        On this page
      </p>
      <ul className="settings-nav-list">
        {items.map((item) => {
          const active = item.id === activeId;
          return (
            <li key={item.id}>
              <a
                href={`#${item.id}`}
                className="settings-nav-link"
                aria-current={active ? "true" : undefined}
                onClick={(e) => handleClick(e, item.id)}
              >
                {item.label}
              </a>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
