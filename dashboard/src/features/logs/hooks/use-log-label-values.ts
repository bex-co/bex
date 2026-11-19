import { useLastAnswer } from "@/common/hooks/use-last-answer";
import { useNow } from "@/common/hooks/use-now";
import { useQuery } from "@apollo/client/react";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { LogLabelValuesDocument } from "@/graphql/definitions";

const HOUR_MS = 60 * 60 * 1000;

/**
 * How far back discovery looks: seven days, the shortest plan's log window
 * (Hobby's), so it never reaches past any workspace's floor. A read without a
 * startTime covers only the last hour, as Render documents (w5/197), and the
 * filter dropdowns would then lose an instance or a status code the App last
 * produced yesterday.
 */
const DISCOVERY_LOOKBACK_MS = 7 * 24 * HOUR_MS;

/**
 * The discovery window's start for the instant `now` (epoch ms): the lookback
 * rounded up to the hour, so the query's variables (and its cache entry)
 * change once an hour rather than on every render, and the start never lies
 * before the seven-day floor.
 */
export function discoveryStartTime(now: number): string {
  return new Date(
    Math.ceil((now - DISCOVERY_LOOKBACK_MS) / HOUR_MS) * HOUR_MS,
  ).toISOString();
}

/**
 * One label's discovery result, with the distinction the filter bar needs:
 * whether the store has actually ANSWERED. "No values" and "no answer" are
 * different facts and the UI must treat them differently (w6/m131/t003).
 */
export interface LogLabelDiscovery {
  /** Values the store reports this App has actually produced. */
  values: string[];
  /**
   * True once discovery has authoritatively answered — the query completed
   * without error, so an empty `values` means "this App has produced none",
   * not "we could not ask". False while loading and when discovery is
   * unavailable (no store => 503), which is exactly when static fallbacks are
   * the honest thing to show.
   */
  resolved: boolean;
}

/**
 * Discovered values for one log label (level/instance/method/statusCode) of an
 * App — bex-api's `logLabelValues` query, backed by the durable store's
 * label-value discovery (docs/ADR010-observability.md § Log filters). Populates the
 * Logs-tab filter dropdowns with values the App has actually produced, not a
 * hardcoded guess. The logs sibling of `useMetricsFilterValues`.
 *
 * Errors degrade to an empty list with `resolved: false`: without the store
 * (local dev, `BEX_LOKI_URL` unset) discovery 503s, so the dropdown offers no
 * discovered values — the static fallbacks the filter bar merges in keep it
 * usable, and picking one surfaces the honest "needs the log store" state on
 * query.
 */
export function useLogLabelDiscovery(
  resource: string,
  label: string,
  { skip = false }: { skip?: boolean } = {},
): LogLabelDiscovery {
  // Discovery keeps up with traffic while the page stays open: a method or
  // status first seen after load becomes selectable on the next visible poll
  // instead of after a reload (w4/178). Apollo's cache-first default would
  // otherwise answer every re-render from the first result forever.
  const startTime = discoveryStartTime(useNow(HOUR_MS));
  const { data, loading, error } = useQuery(LogLabelValuesDocument, {
    variables: { resource, label, startTime },
    errorPolicy: "all",
    skip,
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });

  const answered =
    !loading && !error && data
      ? (data.logLabelValues ?? []).filter((v): v is string => v != null)
      : null;
  // The last authoritative answer for THIS resource+label. A transient poll
  // failure keeps it (stale values beat flipping back to fallbacks or
  // emptying an open picker); only a key that has never answered reports
  // `resolved: false`.
  const values = useLastAnswer(`${resource}\u0000${label}`, answered);
  return values ? { values, resolved: true } : { values: [], resolved: false };
}

/**
 * The values alone, for callers that have no static fallback to reconcile
 * against and so cannot act on the distinction.
 */
export function useLogLabelValues(
  resource: string,
  label: string,
  options: { skip?: boolean } = {},
): string[] {
  return useLogLabelDiscovery(resource, label, options).values;
}
