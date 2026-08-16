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
