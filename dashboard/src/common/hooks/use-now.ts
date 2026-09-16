import { useEffect, useState } from "react";

/**
 * The current clock in epoch ms, re-read every `intervalMs` (default one
 * minute) so elapsed-time text ("Deployed 2 minutes ago") keeps moving on a
 * page left open instead of freezing at its first render. Meant to be called
 * once per page and passed down: twenty rows sharing one timer is one
 * re-render a minute, not twenty timers drifting apart.
 *
 * The initial value is read once per pass — on the server and again during
 * hydration — so any text derived from it must sit under
 * `suppressHydrationWarning`, exactly as `RelativeAge` does (w6/m102).
 */
export function useNow(intervalMs = 60_000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(id);
  }, [intervalMs]);
  return now;
}
