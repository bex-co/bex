import type { ApolloClient } from "@apollo/client";

/**
 * Query defaults every Apollo client in the dashboard shares (w1/m153).
 *
 * Apollo 4 changed `notifyOnNetworkStatusChange` to default to `true`, so a
 * poll or refetch over cached data re-emits `loading: true`. Every consumer
 * that gates a skeleton on `loading` then unmounted the page it already
 * rendered on each 30 s tick, closing open dialogs and dropping typed drafts
 * and focus. `false` stops a background poll, `refetch()` or `fetchMore` from
 * re-announcing `loading` over data already on screen.
 *
 * `loading` can still be true while data is present — mounting a
 * `cache-and-network` query over a warm cache, or switching to variables that
 * are already cached — so a skeleton gate stays `loading && no data yet`.
 *
 * Opt back in with `notifyOnNetworkStatusChange: true` on a query that needs a
 * visible in-flight state, and always on `useLazyQuery`: a lazy query learns
 * of its own request only through that emission, so without it `loading`
 * never turns true when it runs.
 */
export const apolloDefaultOptions = {
  watchQuery: { notifyOnNetworkStatusChange: false },
} satisfies ApolloClient.DefaultOptions;
