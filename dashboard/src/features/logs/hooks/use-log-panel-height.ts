import { useLayoutEffect, useState } from "react";

/** The panel's fixed height before it is measured (server render, tests). */
const VIEWPORT_HEIGHT = 520;
const MIN_FILL_HEIGHT = 320;
// The room left under the panel: the stream status line beneath it and the
// page's bottom padding (the service Logs tab), or, `"flush"`, the page's
// padding alone (a deploy's log), so whatever follows starts at the fold
// rather than being sliced through by it.
const GAP_UNDER_STATUS_LINE = 56;
const GAP_FLUSH = 24;

/** How a panel fills the viewport: not at all (its fixed height), down to the
 *  stream status line under it, or `"flush"` to the page's bottom padding. */
export type LogPanelFill = boolean | "flush";

// Below this width the Logs tab's filter bar sticks under the top bar, and
// the panel is sized to the screen under that docked bar.
export const LOG_DOCK_MEDIA = "(max-width: 39.99rem)";

/**
 * The panel's height once mounted: the viewport's room below the panel's
 * top, so the newest line and the stream status end at the bottom of the
 * screen at every size — no page scroll stacked on the panel's own on a
 * phone, no blank band under a 520px panel on a tall desktop. At least
 * 320px; the server render (and a test without layout) keeps 520px. The
 * loading placeholder uses the same hook, so the two stay one height.
 */
export function useLogPanelHeight<T extends HTMLElement>(
  fill: LogPanelFill = true,
) {
  const enabled = fill !== false;
  const bottomGap = fill === "flush" ? GAP_FLUSH : GAP_UNDER_STATUS_LINE;
  const [el, ref] = useState<T | null>(null);
  const [height, setHeight] = useState(VIEWPORT_HEIGHT);
  useLayoutEffect(() => {
    // A panel its parent sizes (`fill`) needs no measuring or listener.
    if (!el || !enabled) return;
    const scroller = el.closest("[data-page-scroll]");
    const measure = () => {
      // Where the panel sits with the page unscrolled, so a page already
      // scrolled (a phone's docked bar, a short desktop) measures the same.
      let top = el.getBoundingClientRect().top + (scroller?.scrollTop ?? 0);
      // A phone's Logs tab docks its filter bar under the top bar once what
      // sits above it scrolls away (`data-log-dock` inside the panel's
      // `data-log-frame`), so the panel takes the screen under the docked
      // bar — about double what was left beside the header.
      const dock = el
        .closest("[data-log-frame]")
        ?.querySelector<HTMLElement>("[data-log-dock]");
      if (dock && window.matchMedia?.(LOG_DOCK_MEDIA)?.matches) {
        const dockedAt = scroller ? scroller.getBoundingClientRect().top : 0;
        top =
          dockedAt +
          (el.getBoundingClientRect().top - dock.getBoundingClientRect().top);
      }
      if (!(window.innerHeight > 0) || top <= 0) return;
      setHeight(
        Math.max(
          MIN_FILL_HEIGHT,
          Math.round(window.innerHeight - top - bottomGap),
        ),
      );
    };
    measure();
    window.addEventListener("resize", measure);
    // Whatever sits above the panel can change height after it mounts (a
    // resource header settling into more rows): measure again when the page's
    // content resizes, not only the window.
    const content = scroller?.firstElementChild;
    const observer =
      content && typeof ResizeObserver !== "undefined"
        ? new ResizeObserver(measure)
        : null;
    if (content) observer?.observe(content);
    return () => {
      window.removeEventListener("resize", measure);
      observer?.disconnect();
    };
  }, [el, enabled, bottomGap]);
  return { ref, height };
}
