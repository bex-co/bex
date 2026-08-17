/**
 * Warm a route's primary queries from its `loader` so `defaultPreload: "intent"`
 * (`router.tsx`) prefetches them on hover and the page — whose hooks read
 * `PRIMED_FETCH_POLICY` (`cache-first`, w9/m62) — renders from the warm cache on
 * mount instead of showing a post-navigation skeleton (w9/m68).
 *
 * Awaited in parallel: an intent-preload completes during the hover, before the
 * click; a direct navigation (no hover) still lands with data because the loader
 * blocks on it, exactly like the detail routes' title loader. Best-effort — a
 * failed prefetch is swallowed so it never blocks navigation; the component's
 * own hook re-runs and surfaces any error through its `errorPolicy`.
 */
export async function prefetchInParallel(
  thunks: Array<() => Promise<unknown>>,
): Promise<void> {
  await Promise.all(thunks.map((run) => run().catch(() => undefined)));
}
