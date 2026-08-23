import type { WatchQueryFetchPolicy } from "@apollo/client";

/**
 * The mount fetch policy for a query a navigation LANDS on and whose cache is
 * primed ahead of it — by SSR dehydration (`router.tsx` extracts/restores the
 * Apollo cache) or a route-loader `intent` prefetch (w9/m68). `cache-first`
 * reads that warm cache on mount **without** an immediate duplicate network
 * request, so the prefetch/SSR work isn't thrown away; a cold cache still
 * fetches (cache miss), background freshness still arrives on the query's
 * `pollInterval`, and mutations still refresh via `refetchQueries`.
 *
 * Use this on the primary list/detail query of a primed route instead of the
 * default `cache-and-network` (which always refires on mount). w9/m62 t004.
 */
export const PRIMED_FETCH_POLICY: WatchQueryFetchPolicy = "cache-first";

/**
 * The queries every deploy-changing verb must refetch by name.
 *
 * A manual deploy, restart, cancel, or rollback all change three things at
 * once: the deploy history (`Deploys` — the history tab and the header's
 * latest-deploy chip), the events feed (`ServiceEvents`), and the service's own
 * state (`Server` — the header's status pill). `Server` is easy to forget and
 * expensive to omit: it is otherwise only polled every
 * RESOURCE_POLL_INTERVAL_MS, so leaving it out let the header claim "Building"
 * for up to 30s next to a "Canceled" latest-deploy chip on the same page until
 * a reload (w6/m45 t003). Naming the set once is what keeps the verbs from
 * drifting apart again.
 *
 * Named queries match only MOUNTED instances, so nothing inactive is refetched,
 * and Apollo's query deduplication collapses the several mounted `Server`
 * watchers into one request.
 */
export const DEPLOY_REFETCH_QUERIES = ["Server", "Deploys", "ServiceEvents"];
