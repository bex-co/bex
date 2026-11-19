import { useLayoutEffect, useState } from "react";

/** The panel's fixed height before it is measured (server render, tests). */
const VIEWPORT_HEIGHT = 520;
const MIN_FILL_HEIGHT = 320;
// The stream status line under the panel and the page's bottom padding.
const FILL_BOTTOM_GAP = 56;

/**
 * The panel's height once mounted: the viewport's room below the panel's
 * top, so the newest line and the stream status end at the bottom of the
 * screen at every size — no page scroll stacked on the panel's own on a
 * phone, no blank band under a 520px panel on a tall desktop. At least
 * 320px; the server render (and a test without layout) keeps 520px. The
 * loading placeholder uses the same hook, so the two stay one height.
 */
export function useLogPanelHeight<T extends HTMLElement>(enabled = true) {
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
          Math.round(window.innerHeight - top - FILL_BOTTOM_GAP),
        ),
      );
    };
    measure();
    window.addEventListener("resize", measure);
    return () => window.removeEventListener("resize", measure);
  }, [el, enabled]);
  return { ref, height };
}
