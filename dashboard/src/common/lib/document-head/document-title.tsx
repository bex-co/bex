import { useEffect } from "react";
import { DASHBOARD_NAME } from "./constants";

/**
 * A pending post-unmount reconciliation, shared across instances: a mount that
 * immediately follows an unmount (StrictMode's double-invoke, or one fallback
 * page replacing another) supersedes the outgoing cleanup instead of racing it.
 */
let pendingReconcile: ReturnType<typeof setTimeout> | null = null;

/**
 * React 19 hoists fallback titles into the document head during SSR.
 *
 * The unmount reconciliation is the w1/m52 stale-`<title>` fix (inbox 036):
 * when the SSR render served the error page and the client then recovered, the
 * hydration mismatch can leave the server-hoisted error `<title>` behind as an
 * unmanaged head tag — the tab stays on "Something went wrong" even though the
 * page is fine. On unmount, once React's own head mutations have settled, drop
 * any leftover copies of this title and, if nothing else claimed the tab,
 * restore the neutral product name (a recovering route that emits its own
 * title wins automatically — its element is the one we keep).
 *
 * The other half of the recovery loop is `useLoaderErrorRetry`
 * (common/hooks): it re-runs the failed loader and re-commits the head; this
 * cleanup only drops what that recommit can't reach (unmanaged SSR orphans).
 */
export function DashboardDocumentTitle({ title }: { title: string }) {
  useEffect(() => {
    if (pendingReconcile !== null) {
      clearTimeout(pendingReconcile);
      pendingReconcile = null;
    }
    return () => {
      pendingReconcile = setTimeout(() => {
        pendingReconcile = null;
        const titles = Array.from(document.head.querySelectorAll("title"));
        const stale = titles.filter((el) => el.textContent === title);
        if (stale.length === titles.length) {
          // Only this fallback's title (or none) survived — reset the tab.
          // The setter rewrites the first <title> or creates one.
          document.title = DASHBOARD_NAME;
          stale.slice(1).forEach((el) => el.remove());
        } else {
          // A real title exists; drop the orphaned fallback copies.
          stale.forEach((el) => el.remove());
        }
      }, 0);
    };
  }, [title]);
  return <title>{title}</title>;
}
