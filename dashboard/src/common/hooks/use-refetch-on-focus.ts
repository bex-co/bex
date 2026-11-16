import { useEffect } from "react";

/**
 * Runs `refetch` whenever the tab regains focus or becomes visible again:
 * how a change made elsewhere (a plan upgrade in another tab) reaches an open
 * page whose answer otherwise holds still and is read once. A failed read is
 * left to the query's own error state.
 */
export function useRefetchOnFocus(refetch: () => unknown): void {
  useEffect(() => {
    const onFocus = () => {
      if (document.visibilityState === "visible") void refetch();
    };
    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onFocus);
    return () => {
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onFocus);
    };
  }, [refetch]);
}
