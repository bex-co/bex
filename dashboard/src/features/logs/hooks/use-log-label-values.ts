import { useQuery } from "@apollo/client/react";
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
  const { data, loading, error } = useQuery(LogLabelValuesDocument, {
    variables: { resource, label },
    errorPolicy: "all",
  });

  return {
    values: (data?.logLabelValues ?? []).filter((v): v is string => v != null),
    resolved: !loading && !error,
  };
}

/**
 * The values alone, for callers that have no static fallback to reconcile
 * against and so cannot act on the distinction.
 */
export function useLogLabelValues(resource: string, label: string): string[] {
  return useLogLabelDiscovery(resource, label).values;
}
