import { useEffect, useRef } from "react";

/**
 * Runs `refetch` whenever `key` changes after the first render (not on
 * mount, where the query has just read). `enabled` false skips the read but
 * still takes note of the new key. A failed read is left to the query's own
 * error state.
 */
export function useRefetchOnChange(
  key: string | undefined,
  refetch: () => Promise<unknown>,
  enabled = true,
): void {
  const lastKey = useRef(key);
  useEffect(() => {
    if (lastKey.current === key) return;
    lastKey.current = key;
    if (enabled) void refetch().catch(() => undefined);
  }, [key, enabled, refetch]);
}
