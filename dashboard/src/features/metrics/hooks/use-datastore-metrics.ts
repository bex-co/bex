import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { isThrottledError } from "@/common/lib/graphql-error";
import { isLogStoreUnavailable } from "@/features/logs/lib/log-store";
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
 * Reads one bex-api datastore metric (disk, connections, replication lag,
 * the database-activity and disk graphs, …) for one managed Postgres, Key
 * Value or service disk via GraphQL — the
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
    pollIntervalMs = RESOURCE_POLL_INTERVAL_MS,
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
    skipPollAttempt: skipPollWhenHidden,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const unavailable = isMetricsUnavailable(error);
  // Lock-Delayed Queries is counted from Postgres log lines (w7/m430), so it
  // answers LOG_STORE_UNAVAILABLE on a deployment without the log store.
  const storeUnavailable = !unavailable && isLogStoreUnavailable(error);
  const throttled =
    !unavailable && !storeUnavailable && isThrottledError(error);

  const series: ChartSeries[] = useMemo(
    () => toChartSeries(data?.datastoreMetrics),
    [data],
  );

  return {
    series,
    loading,
    unavailable,
    storeUnavailable,
    throttled,
    error: unavailable || storeUnavailable || throttled ? undefined : error,
    // Datastore metrics never carry the bandwidth degraded_sources label.
    degradedSources: EMPTY_DEGRADED,
  };
}

// Stable identity so poll-tick re-renders don't churn consumers' memos.
const EMPTY_DEGRADED: string[] = [];
