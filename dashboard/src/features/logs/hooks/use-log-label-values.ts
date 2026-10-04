import { useState } from "react";
import { useQuery } from "@apollo/client/react";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { LogLabelValuesDocument } from "@/graphql/definitions";

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
): LogLabelDiscovery {
  // Discovery keeps up with traffic while the page stays open: a method or
  // status first seen after load becomes selectable on the next visible poll
  // instead of after a reload (w4/178). Apollo's cache-first default would
  // otherwise answer every re-render from the first result forever.
  const { data, loading, error } = useQuery(LogLabelValuesDocument, {
    variables: { resource, label },
    errorPolicy: "all",
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
  // `resolved: false`. Adjusted during render, React's pattern for state
  // derived from props.
  const key = `${resource}\u0000${label}`;
  const [last, setLast] = useState<{ key: string; values: string[] } | null>(
    null,
  );
  if (answered && (last?.key !== key || !sameValues(last.values, answered))) {
    setLast({ key, values: answered });
  }
  if (answered) return { values: answered, resolved: true };
  if (last?.key === key) return { values: last.values, resolved: true };
  return { values: [], resolved: false };
}

function sameValues(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((v, i) => v === b[i]);
}

/**
 * The values alone, for callers that have no static fallback to reconcile
 * against and so cannot act on the distinction.
 */
export function useLogLabelValues(resource: string, label: string): string[] {
  return useLogLabelDiscovery(resource, label).values;
}
