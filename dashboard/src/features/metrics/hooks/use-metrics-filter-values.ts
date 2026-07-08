import { useQuery } from "@apollo/client/react";
import { MetricsFiltersDocument } from "@/graphql/definitions";

/**
 * Discovered values for one output-filter field (e.g. STATUS_CODE) of an App's
 * metrics — bex-api's `metricsFilters` query, backed by Prometheus label-value
 * discovery (docs/observability.md). Populates Render's toolbar filter
 * dropdowns with codes the App has actually returned, not a hardcoded guess.
 * Errors (including "source not configured") degrade to an empty list — the
 * dropdown just offers no discovered values.
 */
export function useMetricsFilterValues(
  resource: string,
  field: "STATUS_CODE" | "HOST",
): string[] {
  const { data } = useQuery(MetricsFiltersDocument, {
    variables: {
      query: {
        filters: [{ field: "RESOURCE", values: [resource] }],
        outputFilters: [field],
      },
    },
    errorPolicy: "all",
  });

  return (
    data?.metricsFilters?.values
      ?.find((v) => v?.field === field)
      ?.values?.filter((v): v is string => v != null) ?? []
  );
}
