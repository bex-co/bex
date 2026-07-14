import { useQuery } from "@apollo/client/react";
import { LogLabelValuesDocument } from "@/graphql/definitions";

/**
 * Discovered values for one log label (level/instance/method/statusCode) of an
 * App — bex-api's `logLabelValues` query, backed by the durable store's
 * label-value discovery (docs/ADR010-observability.md § Log filters). Populates the
 * Logs-tab filter dropdowns with values the App has actually produced, not a
 * hardcoded guess. The logs sibling of `useMetricsFilterValues`.
 *
 * Errors degrade to an empty list: without the store (local dev, `BEX_LOKI_URL`
 * unset) discovery 503s, so the dropdown simply offers no discovered values —
 * the static fallbacks the filter bar merges in keep it usable, and picking one
 * surfaces the honest "needs the log store" state on query.
 */
export function useLogLabelValues(resource: string, label: string): string[] {
  const { data } = useQuery(LogLabelValuesDocument, {
    variables: { resource, label },
    errorPolicy: "all",
  });

  return (data?.logLabelValues ?? []).filter((v): v is string => v != null);
}
