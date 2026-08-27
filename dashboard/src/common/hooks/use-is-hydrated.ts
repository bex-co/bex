import { useSyncExternalStore } from "react";

// A permanently-idle store: the snapshot never changes after the initial
// server→client transition, so there is nothing to subscribe to.
const subscribe = () => () => {};
const getClientSnapshot = () => true;
const getServerSnapshot = () => false;

/**
 * `false` during the SSR pass and the first (hydrating) client render, then
 * `true` for every render after hydration commits. `useSyncExternalStore` drives
 * this on purpose: React uses `getServerSnapshot` for the hydration render, so
 * SSR and the first client render agree byte-for-byte (no React #418), then
 * switches to `getClientSnapshot` and re-renders once — the officially-supported
 * way to defer inherently client-only output (the viewer's timezone, locale,
 * `window`) without a hydration mismatch.
 *
 * Use this to gate anything the server cannot know the client's value of. For
 * absolute timestamps, prefer the `LocalDateTime`/`LocalDate` wrappers
 * (`common/components/local-time.tsx`) which bake this in.
 */
export function useIsHydrated(): boolean {
  return useSyncExternalStore(
    subscribe,
    getClientSnapshot,
    getServerSnapshot,
  );
}
