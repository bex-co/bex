import { useLayoutEffect, type RefObject } from "react";
import { LOG_DOCK_MEDIA } from "@/features/logs/hooks/use-log-panel-height";

/**
 * On a phone the Logs tab opens docked: its filter bar (the frame's first
 * child) under the top bar and the log filling the screen below, the newest
 * line and the stream status in view. The log is sized for that docked view,
 * so opening with the service header above it left the newest lines a screen
 * below the fold; scrolling up still brings the header back. Before paint, so
 * nothing jumps; the loading frame docks the same way, so the lines land where
 * it stood.
 *
 * The frame is held tall enough that the page's end is that docked view: the
 * log sets its height in its own layout effect, after this one, so without it
 * the page could not yet scroll the bar all the way up; and no further, so
 * scrolling on never lifts the log from under the bar. It follows the
 * scroller's size (a `ResizeObserver`, which reports after layout, where a
 * window resize event can read the old height).
 */
export function useOpenDocked(frameRef: RefObject<HTMLElement | null>) {
  useLayoutEffect(() => {
    const frame = frameRef.current;
    const scroller = frame?.closest<HTMLElement>("[data-page-scroll]");
    if (!frame || !scroller) return;
    const fit = () => {
      if (!window.matchMedia?.(LOG_DOCK_MEDIA)?.matches) {
        frame.style.minHeight = "";
        return false;
      }
      // The page's own space under the frame (its bottom padding).
      const box = scroller.getBoundingClientRect();
      const below =
        scroller.scrollHeight -
        (frame.getBoundingClientRect().bottom - box.top + scroller.scrollTop);
      frame.style.minHeight = `${Math.max(0, scroller.clientHeight - below)}px`;
      return true;
    };
    if (fit()) {
      scroller.scrollTop +=
        frame.getBoundingClientRect().top -
        scroller.getBoundingClientRect().top;
    }
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(() => fit());
    observer.observe(scroller);
    return () => observer.disconnect();
  }, [frameRef]);
}
