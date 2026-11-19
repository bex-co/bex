import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { isThrottledError } from "@/common/lib/graphql-error";
import { ProjectMetricsDocument } from "@/graphql/definitions";
import {
  RENDER_METRIC_NAMES,
  type ChartSeries,
  type MetricId,
} from "@/features/metrics/types";
import {
  isMetricsUnavailable,
  toChartSeries,
} from "@/features/metrics/lib/graphql-series";
import type {
  ProjectMetricGroupBy,
  ProjectMetricResource,
  ProjectMetricsScope,
} from "@/features/metrics/lib/project-metrics";
import type { UseMetricsResult } from "./use-metrics";

export interface UseProjectMetricsOptions {
  startTime?: string;
  endTime?: string;
  resolutionSeconds?: number;
  /** The http_latency percentile. */
  quantile?: number;
  groupBy?: ProjectMetricGroupBy;
  /** 0 (the page's live window refetches by moving) disables polling. */
  pollIntervalMs?: number;
  skip?: boolean;
}

export interface UseProjectMetricsResult extends UseMetricsResult {
  /** Every resource of the view the caller may read, and what this chart
   *  did with it. Empty until the first answer. */
  resources: ProjectMetricResource[];
}

const NO_RESOURCES: ProjectMetricResource[] = [];

/**
 * One chart of a project's Service Metrics view (w3/m508): bex-api reads every
 * resource the view's filter document keeps in one request. The result has
 * `useMetrics`' shape, so a chart section shows the same loading, unavailable,
 * throttled and plan states as a service page's.
 */
export function useProjectMetrics(
  scope: ProjectMetricsScope,
  metric: MetricId,
  opts: UseProjectMetricsOptions = {},
): UseProjectMetricsResult {
  const {
    startTime,
    endTime,
    resolutionSeconds,
    quantile,
    groupBy,
    pollIntervalMs = 0,
    skip = false,
  } = opts;
  const { data, loading, error } = useQuery(ProjectMetricsDocument, {
    variables: {
      query: {
        project: scope.project,
        filterDocument: scope.filterDocument,
        name: RENDER_METRIC_NAMES[metric],
        start: startTime,
        end: endTime,
        resolution: resolutionSeconds,
        parameters: quantile != null ? [{ quantile }] : undefined,
        groupBy,
      },
    },
    skip,
    pollInterval: pollIntervalMs,
    skipPollAttempt: skipPollWhenHidden,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const unavailable = isMetricsUnavailable(error);
  const throttled = !unavailable && isThrottledError(error);
  const answer = data?.projectMetrics;
  const series: ChartSeries[] = useMemo(
    () => toChartSeries(answer?.series),
    [answer],
  );
  return {
    series,
    resources: answer?.resources ?? NO_RESOURCES,
    loading,
    unavailable,
    storeUnavailable: false,
    throttled,
    error: unavailable || throttled ? undefined : error,
    degradedSources: [],
  };
}
