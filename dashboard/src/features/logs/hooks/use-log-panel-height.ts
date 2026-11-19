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
    const measure = () => {
      const top = el.getBoundingClientRect().top;
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
    return () => window.removeEventListener("resize", measure);
  }, [el, enabled, bottomGap]);
  return { ref, height };
}
