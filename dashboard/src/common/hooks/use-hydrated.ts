import { useSyncExternalStore } from "react";

const subscribe = () => () => {};

/**
 * False on the server and during hydration, true once the page has hydrated.
 * A state the client can already decide from data the server render could
 * not see (a query the stream delivered after the shell) waits for it, so
 * hydration matches the server's HTML and the state draws on the next render
 * (w3/m706: a dead deploy's not-found page).
 */
export function useHydrated(): boolean {
  return useSyncExternalStore(
    subscribe,
    () => true,
    () => false,
  );
}
