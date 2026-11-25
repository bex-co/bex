import { useEffect, useRef, useState } from "react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/common/lib/utils/utils";

export interface SectionNavigationItem {
  href: string;
  label: string;
  icon: LucideIcon;
}

/*
 * Section navigation has two forms, chosen by the width of the page's own
 * content area (the `@container/page` the page scroller declares), not by the
 * viewport: the sidebar is resizable and collapsible, so a 970px window can
 * hold a 700px page or a 920px one.
 *
 *   - Below 56rem of content (`@4xl/page`): a tab bar under the page header.
 *     It sticks flush beneath the app header — its `top` cancels the
 *     scroller's top padding, published as `--page-pad-top` — bleeds to the
 *     scroller's edges, and marks the current section with a filled pill — not
 *     an underline: a detail page's own tabs (Overview / Logs) are underlined
 *     right above it, and in-page jump links must not read as a second row of
 *     page tabs.
 *   - From 56rem: a right rail beside the content, 24px below the scrollport
 *     top, marking the current section with a filled row.
 *
 * Every surface with section navigation (PageFrame, service settings, the
 * database and Key Value detail pages, their skeletons) uses these classes, so
 * the two forms cannot drift apart between pages.
 */

/**
 * The grid: one column of content, plus the rail column when wide. The narrow
 * column is an explicit minmax(0,1fr): an implicit `auto` column grows to the
 * tab bar's full scroll width and pushes the whole page off-screen. When
 * wide, the content keeps a form's reading measure (48rem) and the rail
 * sits at the page's right edge, under the header's actions; the rail is
 * wide enough for its longest entry on one line
 * ("Image registry credential" wrapped at 13rem).
 */
export const SECTIONED_GRID_CLASS =
  "grid w-full grid-cols-[minmax(0,1fr)] items-start gap-6 @4xl/page:grid-cols-[minmax(0,48rem)_15rem] @4xl/page:justify-between @4xl/page:gap-x-10";

/** The content column, beside the rail when wide. It is a container too, so
 *  a card inside lays itself out by the column it actually has. */
export const SECTIONED_BODY_CLASS =
  "@container min-w-0 space-y-6 @4xl/page:col-start-1";

/**
 * The navigation's wrapper. Narrow: the sticky bar — flush under the app
 * header (its top cancels the scroller's top padding), bled to the scroller's
 * edges, translucent over scrolling content. Wide: the sticky rail, 24px
 * below the scrollport top, in the grid's second column.
 */
export const SECTION_NAVIGATION_STICKY_CLASS =
  "sticky top-[calc(var(--page-pad-top,0px)*-1)] z-20 -mx-4 min-w-0 border-b bg-background px-4 sm:-mx-6 sm:px-6 supports-[backdrop-filter]:bg-background/85 supports-[backdrop-filter]:backdrop-blur-md @4xl/page:top-[calc(1.5rem_-_var(--page-pad-top,0px))] @4xl/page:col-start-2 @4xl/page:row-start-1 @4xl/page:mx-0 @4xl/page:border-0 @4xl/page:bg-transparent @4xl/page:px-0 @4xl/page:supports-[backdrop-filter]:bg-transparent @4xl/page:supports-[backdrop-filter]:backdrop-blur-none";

/**
 * Scroll margin for a section a link jumps to. Under the bar (48px + its
 * border, flush at the scrollport top) the heading lands 24px below it;
 * beside the rail it lands 24px below the scrollport top, level with the rail.
 */
export const SECTION_NAVIGATION_TARGET_CLASS =
  "scroll-mt-[4.5625rem] @4xl/page:scroll-mt-6";

/** The links' list: a scrolling row of pills, or the rail's column. -mx-3
 *  lines the first pill's label up with the content's left edge. */
export const SECTION_NAVIGATION_ITEMS_CLASS =
  "-mx-3 flex gap-1 overflow-x-auto py-2 scrollbar-none @4xl/page:mx-0 @4xl/page:flex-col @4xl/page:gap-0.5 @4xl/page:overflow-visible @4xl/page:py-0";

/** A pill in the bar; a full-width row in the rail. Current: filled. */
const LINK_CLASS =
  "flex h-8 shrink-0 items-center gap-2 rounded-full px-3 max-lg:h-11 pointer-coarse:h-11 text-sm font-medium whitespace-nowrap text-muted-foreground transition-colors hover:bg-accent/60 hover:text-foreground aria-[current=location]:bg-accent aria-[current=location]:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset @4xl/page:h-auto @4xl/page:rounded-md @4xl/page:py-2 @4xl/page:whitespace-normal @4xl/page:hover:bg-accent";

/** Fade the bar's clipped edge(s) so overflow reads as "more this way". */
const FADE_CLASS: Record<"start" | "end" | "both", string> = {
  end: "[mask-image:linear-gradient(to_right,#000_calc(100%_-_2.5rem),transparent)]",
  start:
    "[mask-image:linear-gradient(to_left,#000_calc(100%_-_2.5rem),transparent)]",
  both: "[mask-image:linear-gradient(to_right,transparent,#000_2.5rem,#000_calc(100%_-_2.5rem),transparent)]",
};

/** How far below the scrollport's top a section's heading counts as "here". */
const ACTIVE_OFFSET_PX = 120;

/** The page's scroll container: the frame's, else the nearest scrolling box. */
function scrollParent(node: HTMLElement): HTMLElement | null {
  const frame = node.closest<HTMLElement>("[data-page-scroll]");
  if (frame) return frame;
  for (let el = node.parentElement; el; el = el.parentElement) {
    if (/(auto|scroll)/.test(getComputedStyle(el).overflowY)) return el;
  }
  return null;
}

/**
 * Scroll-spy for the section rail: which section the reader is in, and which
 * sections rendered nothing (a conditional card that does not apply) so the
 * rail does not point at empty space. Client-only — the server render and the
 * hydration pass have no active item and hide nothing.
 */
function useSectionState(
  hrefs: string[],
  navRef: React.RefObject<HTMLElement | null>,
) {
  const [active, setActive] = useState<string | null>(null);
  const [empty, setEmpty] = useState<ReadonlySet<string>>(() => new Set());
  // A clicked section holds the highlight while the smooth scroll travels
  // there, so the sections it passes don't flicker through the bar.
  const pinned = useRef<{ href: string; until: number } | null>(null);
  const key = hrefs.join("|");

  useEffect(() => {
    const nav = navRef.current;
    if (!nav) return;
    const scroller = scrollParent(nav);
    let frame = 0;

    const update = () => {
      frame = 0;
      const top = scroller ? scroller.getBoundingClientRect().top : 0;
      const present: string[] = [];
      const nowEmpty = new Set<string>();
      let current: string | null = null;
      // No layout (jsdom, a hidden tab): measure nothing, hide nothing.
      const navBox = nav.getBoundingClientRect();
      const hasLayout = navBox.width > 0 || navBox.height > 0;
      for (const href of hrefs) {
        const el = document.getElementById(href.slice(1));
        // A section not mounted yet (still loading) keeps its link; one that
        // mounted and drew nothing (a card that does not apply) loses it.
        if (!el) continue;
        const box = el.getBoundingClientRect();
        if (hasLayout && box.height === 0) {
          nowEmpty.add(href);
          continue;
        }
        present.push(href);
        if (box.top - top <= ACTIVE_OFFSET_PX) {
          current = href;
        }
      }
      const atBottom = scroller
        ? scroller.scrollTop > 0 &&
          scroller.scrollTop + scroller.clientHeight >=
            scroller.scrollHeight - 4
        : false;
      if (atBottom && present.length > 0) current = present.at(-1)!;
      const pin = pinned.current;
      if (pin && performance.now() < pin.until) current = pin.href;
      else pinned.current = null;
      setActive(current ?? present[0] ?? null);
      setEmpty((previous) =>
        previous.size === nowEmpty.size &&
        [...nowEmpty].every((href) => previous.has(href))
          ? previous
          : nowEmpty,
      );
    };
    const schedule = () => {
      if (!frame) frame = requestAnimationFrame(update);
    };

    update();
    const target: HTMLElement | Window = scroller ?? window;
    target.addEventListener("scroll", schedule, { passive: true });
    // With a scroller, its observed content box covers window resizes too.
    if (!scroller) window.addEventListener("resize", schedule);
    // Sections settle after their data loads; re-measure when the content
    // column changes size.
    const observer =
      scroller && typeof ResizeObserver !== "undefined"
        ? new ResizeObserver(schedule)
        : null;
    if (scroller?.firstElementChild)
      observer?.observe(scroller.firstElementChild);
    return () => {
      if (frame) cancelAnimationFrame(frame);
      target.removeEventListener("scroll", schedule);
      window.removeEventListener("resize", schedule);
      observer?.disconnect();
    };
    // `key` stands for `hrefs`, which callers rebuild every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, navRef]);

  const pin = (href: string) => {
    pinned.current = { href, until: performance.now() + 1000 };
    setActive(href);
  };

  return { active, pin, empty };
}

/** How long a section the URL names is held in place while the page loads. */
const HASH_HOLD_MS = 5000;

/** What counts as the reader taking over the scroll position. */
const READER_INPUT_EVENTS = [
  "wheel",
  "touchstart",
  "pointerdown",
  "keydown",
] as const;

/**
 * Holds the section the URL's hash names (`/settings#api-keys`) at the top
 * of the page while the content above it settles. The router scrolls to the
 * hash once, as the page renders, and cards that load after it then move the
 * section away: Workspace Settings' Agents cards shrink by about 2,000px
 * once they load, which left `#registry-credentials` scrolled past (w4/711).
 * Every content resize re-aligns the section until the reader scrolls,
 * clicks or presses a key, or HASH_HOLD_MS passes.
 */
function useHoldHashTarget(navRef: React.RefObject<HTMLElement | null>) {
  useEffect(() => {
    const id = window.location.hash.slice(1);
    const nav = navRef.current;
    if (!id || !nav || typeof ResizeObserver === "undefined") return;
    const scroller = scrollParent(nav);
    const content = scroller?.firstElementChild;
    if (!scroller || !content) return;

    const observer = new ResizeObserver(() => {
      const target = document.getElementById(id);
      if (target && scroller.contains(target)) {
        target.scrollIntoView({ block: "start" });
      }
    });
    let timer = 0;
    const release = () => {
      observer.disconnect();
      window.clearTimeout(timer);
      for (const type of READER_INPUT_EVENTS) {
        window.removeEventListener(type, release, true);
      }
    };
    observer.observe(content);
    timer = window.setTimeout(release, HASH_HOLD_MS);
    for (const type of READER_INPUT_EVENTS) {
      window.addEventListener(type, release, { capture: true, passive: true });
    }
    return release;
  }, [navRef]);
}

/**
 * Responsive in-page section navigation shared by long settings surfaces: a
 * tab bar on a narrow page, a right rail on a wide one (see the classes
 * above). It owns the navigation semantics, the current-section highlight,
 * the tab bar's overflow affordance and link presentation; the owning layout
 * places it with SECTION_NAVIGATION_STICKY_CLASS.
 */
export function SectionNavigation({
  ariaLabel,
  items,
  className,
}: {
  ariaLabel: string;
  items: SectionNavigationItem[];
  className?: string;
}) {
  const navRef = useRef<HTMLElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const { active, pin, empty } = useSectionState(
    items.map((item) => item.href),
    navRef,
  );
  useHoldHashTarget(navRef);

  const visible = items.filter(({ href }) => !empty.has(href));
  const visibleKey = visible.map(({ href }) => href).join("|");

  // In the bar, keep the current section's link in view, and fade
  // whichever edge has more tabs beyond it.
  const [fade, setFade] = useState<keyof typeof FADE_CLASS | null>(null);
  useEffect(() => {
    const list = listRef.current;
    if (!list) return;
    const measure = () => {
      const overflow = list.scrollWidth - list.clientWidth;
      if (overflow <= 1) return setFade(null);
      const atStart = list.scrollLeft <= 1;
      const atEnd = list.scrollLeft >= overflow - 1;
      setFade(atStart ? "end" : atEnd ? "start" : "both");
    };
    measure();
    list.addEventListener("scroll", measure, { passive: true });
    const observer =
      typeof ResizeObserver !== "undefined"
        ? new ResizeObserver(measure)
        : null;
    // The bar's box rarely changes; what overflows it does — links hidden
    // once their section turns out empty, labels re-flowing as fonts load.
    observer?.observe(list);
    for (const link of list.children) observer?.observe(link);
    return () => {
      list.removeEventListener("scroll", measure);
      observer?.disconnect();
    };
  }, [visibleKey]);

  useEffect(() => {
    const list = listRef.current;
    if (!list || !active || list.scrollWidth <= list.clientWidth) return;
    const link = list.querySelector<HTMLElement>(`a[href="${active}"]`);
    if (!link) return;
    const box = link.getBoundingClientRect();
    const start =
      box.left - list.getBoundingClientRect().left + list.scrollLeft;
    const end = start + box.width;
    const inset = 40; // the fade's width: keep the tab clear of it
    if (start < list.scrollLeft + inset)
      list.scrollTo({ left: start - inset, behavior: "smooth" });
    else if (end > list.scrollLeft + list.clientWidth - inset)
      list.scrollTo({
        left: end - list.clientWidth + inset,
        behavior: "smooth",
      });
  }, [active]);

  return (
    <nav
      ref={navRef}
      aria-label={ariaLabel}
      className={cn("min-w-0", className)}
    >
      <div
        ref={listRef}
        className={cn(
          SECTION_NAVIGATION_ITEMS_CLASS,
          fade && FADE_CLASS[fade],
          "@4xl/page:[mask-image:none]",
        )}
      >
        {visible.map(({ href, label, icon: Icon }) => (
          <a
            key={href}
            href={href}
            aria-current={href === active ? "location" : undefined}
            onClick={(event) => {
              const target = document.getElementById(href.slice(1));
              if (!target?.scrollIntoView) return; // plain anchor jump
              event.preventDefault();
              pin(href);
              const reduce = window.matchMedia?.(
                "(prefers-reduced-motion: reduce)",
              ).matches;
              target.scrollIntoView({
                behavior: reduce ? "auto" : "smooth",
                block: "start",
              });
              history.replaceState(history.state, "", href);
            }}
            className={LINK_CLASS}
          >
            <Icon
              aria-hidden="true"
              className="hidden size-4 shrink-0 @4xl/page:block"
            />
            {label}
          </a>
        ))}
      </div>
    </nav>
  );
}
