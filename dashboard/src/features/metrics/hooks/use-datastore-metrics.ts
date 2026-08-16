import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { DatastoreMetricsDocument } from "@/graphql/definitions";
import {
  RENDER_DATASTORE_METRIC_NAMES,
  type DatastoreKind,
  type DatastoreMetricId,
  type ChartSeries,
} from "@/features/metrics/types";
import {
  isMetricsUnavailable,
  toChartSeries,
} from "@/features/metrics/lib/graphql-series";
import type {
  UseMetricsOptions,
  UseMetricsResult,
} from "@/features/metrics/hooks/use-metrics";

/**
 * Reads one bex-api datastore metric (disk/db_connections/replication_lag)
 * for one managed Postgres or Key Value instance via GraphQL — the
 * Database/KeyValue-scoped sibling of useMetrics (w3/m10). Unlike useMetrics'
 * RESOURCE filter array, datastoreMetrics names one resource + kind directly.
 */
export function useDatastoreMetrics(
  kind: DatastoreKind,
  resource: string,
  metric: DatastoreMetricId,
  opts: Pick<
    UseMetricsOptions,
    "startTime" | "endTime" | "resolutionSeconds" | "pollIntervalMs"
  > & {
    /** Skip the query entirely — e.g. a metric that doesn't apply to `kind`. */
    skip?: boolean;
  } = {},
): UseMetricsResult {
  const {
    pollIntervalMs = 30_000,
    startTime,
    endTime,
    resolutionSeconds,
    skip = false,
  } = opts;

  const { data, loading, error } = useQuery(DatastoreMetricsDocument, {
    variables: {
      query: {
        kind,
        resource,
        name: RENDER_DATASTORE_METRIC_NAMES[metric],
        start: startTime,
        end: endTime,
        resolution: resolutionSeconds,
      },
    },
    skip,
    pollInterval: pollIntervalMs,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const unavailable = isMetricsUnavailable(error);

  const series: ChartSeries[] = useMemo(
    () => toChartSeries(data?.datastoreMetrics),
    [data],
  );

  return {
    series,
    loading,
    unavailable,
    // Datastore metrics take no host/path filter, so the log-store-unavailable
    // state (w5/m58) never applies here.
    storeUnavailable: false,
    error: unavailable ? undefined : error,
    // Datastore metrics never carry the bandwidth degraded_sources label.
    degradedSources: EMPTY_DEGRADED,
  };
}

// Stable identity so poll-tick re-renders don't churn consumers' memos.
const EMPTY_DEGRADED: string[] = [];
