import { useCallback, useRef, useState } from "react";

/**
 * A failed read's in-place Retry (plan catalogs w4/m218, placement lists
 * w4/m221). The app's queries keep
 * `notifyOnNetworkStatusChange` off so a refetch never unmounts a draft, which
 * also means `loading` stays false during one — so the busy state is tracked
 * here. A second click while a retry is in flight starts nothing; a failed
 * retry leaves the query's own error to say so.
 */
export function useCatalogRetry(refetch: () => Promise<unknown>): {
  retry: () => Promise<void>;
  retrying: boolean;
} {
  const [retrying, setRetrying] = useState(false);
  const inFlight = useRef(false);
  const retry = useCallback(async () => {
    if (inFlight.current) return;
    inFlight.current = true;
    setRetrying(true);
    try {
      await refetch();
    } catch {
      // The query's error state carries the failure.
    } finally {
      inFlight.current = false;
      setRetrying(false);
    }
  }, [refetch]);
  return { retry, retrying };
}
