import { useCallback, useEffect, useRef, useState } from "react";

/**
 * Sticky in-page section index for the Settings page.
 *
 * The settings form spans ~2400px of vertical scroll across seven
 * sections. Without a navigation surface the operator's only path
 * to a specific knob is "scroll and scan." This component renders
 * a sticky list of section anchors and tracks which section heading
 * has crossed the activation line below the sticky app header.
 *
 * Visibility is desktop-only: at <1024px the page falls back to
 * vertical scroll without a nav rail (see the matching
 * @media rule in settings.css).
 */
export type SettingsNavItem = {
  /** DOM id of the section element. */
  id: string;
  /** Sentence-case label rendered in the rail. */
  label: string;
};

/** Aligns with `.settings-section` / phase panel `scroll-margin-top`. */
const ACTIVATION_TOP_PX = 96;

/** Ignore scroll-driven updates while smooth scroll from a nav click runs. */
const CLICK_LOCK_MS = 900;

export function SettingsNav({ items }: { items: SettingsNavItem[] }) {
  const [activeId, setActiveId] = useState<string>(items[0]?.id ?? "");
  const lastActiveRef = useRef<string>(activeId);
  const clickLockUntilRef = useRef(0);
  const rafRef = useRef<number | null>(null);

  const computeActiveId = useCallback((): string => {
    if (items.length === 0) return "";

    const doc = document.documentElement;
    const atBottom =
      window.innerHeight + window.scrollY >= doc.scrollHeight - 2;
    if (atBottom) {
      return items[items.length - 1].id;
    }

    let active = items[0].id;
    for (const item of items) {
      const el = document.getElementById(item.id);
      if (!el) continue;
      if (el.getBoundingClientRect().top <= ACTIVATION_TOP_PX) {
        active = item.id;
      }
    }
    return active;
  }, [items]);

  const syncActive = useCallback(() => {
    if (Date.now() < clickLockUntilRef.current) return;
    const next = computeActiveId();
    if (next === lastActiveRef.current) return;
    lastActiveRef.current = next;
    setActiveId(next);
  }, [computeActiveId]);

  const scheduleSync = useCallback(() => {
    if (rafRef.current !== null) return;
    rafRef.current = window.requestAnimationFrame(() => {
      rafRef.current = null;
      syncActive();
    });
  }, [syncActive]);

  useEffect(() => {
    syncActive();

    window.addEventListener("scroll", scheduleSync, {
      passive: true,
      capture: true,
    });
    window.addEventListener("resize", scheduleSync, { passive: true });
    return () => {
      window.removeEventListener("scroll", scheduleSync, true);
      window.removeEventListener("resize", scheduleSync);
      if (rafRef.current !== null) {
        window.cancelAnimationFrame(rafRef.current);
      }
    };
  }, [scheduleSync, syncActive]);

  function handleClick(e: React.MouseEvent<HTMLAnchorElement>, id: string) {
    e.preventDefault();
    const el = document.getElementById(id);
    if (!el) return;
    const prefersReduced =
      typeof window.matchMedia === "function" &&
      window.matchMedia("(prefers-reduced-motion: reduce)").matches;

    lastActiveRef.current = id;
    setActiveId(id);
    if (!prefersReduced) {
      clickLockUntilRef.current = Date.now() + CLICK_LOCK_MS;
    }

    el.scrollIntoView({
      behavior: prefersReduced ? "auto" : "smooth",
      block: "start",
    });

    if (prefersReduced) {
      syncActive();
    }
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
