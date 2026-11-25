import { useMemo, useState, type ReactNode } from "react";
import { AlertTriangle } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { EmptyState } from "@/common/components/empty-state";
import { useTranslations } from "@/common/hooks/use-translations";
import { graphQLErrorExtensions } from "@/common/lib/graphql-error";
import { cn } from "@/common/lib/utils/utils";
import { useWorkspace } from "@/features/workspaces/context";
import { proFeaturesPlan } from "@/features/workspaces/types";
import { PROJECT_LEGEND_ROW_CLASS } from "@/features/metrics/components/chart-layout";
import { MetricSection } from "@/features/metrics/components/metric-section";
import { RangeSelect } from "@/features/metrics/components/range-select";
import {
  ChartLegend,
  SvgLineChart,
} from "@/features/metrics/components/svg-line-chart";
import { useLiveRange } from "@/features/metrics/hooks/use-live-range";
import { useMetricsRetentionBound } from "@/features/metrics/hooks/use-metrics-retention";
import {
  useProjectMetrics,
  type UseProjectMetricsResult,
} from "@/features/metrics/hooks/use-project-metrics";
import type { RangeSelection } from "@/features/metrics/lib/range";
import {
  projectChartLines,
  projectChartNotes,
  projectLegend,
  type ProjectLegendEntry,
  type ProjectMetricGroupBy,
  type ProjectMetricsScope,
} from "@/features/metrics/lib/project-metrics";

/** bex-api's refusal of a view past the resources one read charts. */
const PROJECT_VIEW_TOO_LARGE = "PROJECT_VIEW_TOO_LARGE";

// Render's Percentile options on Response Times (w5/m42), p90 its default.
// One line per resource each, so the "All" overlay a service page offers
// would triple them; it is left out here.
const PERCENTILES = [
  { value: "0.5", label: "p50" },
  { value: "0.75", label: "p75" },
  { value: "0.9", label: "p90" },
  { value: "0.99", label: "p99" },
];
const DEFAULT_PERCENTILE = "0.9";

// bex-api refuses a bandwidth step below one minute (minBandwidthStep).
const MIN_BANDWIDTH_RESOLUTION_SECONDS = 60;

export interface ProjectMetricsViewProps {
  scope: ProjectMetricsScope;
  range: RangeSelection;
  onRangeChange: (range: RangeSelection) => void;
}

/**
 * The charts of a project's Service Metrics view (w3/m508), as Render's docs
 * describe the page ("Projects and environments display metrics for all of
 * their services"): one line per resource on CPU, Memory and Total Instances,
 * Total Requests, Response Times and Outbound Bandwidth, with a legend that
 * hides and shows a resource on every chart. Total Requests groups by
 * resource, status code or none, Render's metrics-view group-by. Under a chart
 * it names the resources it could not draw: a failed read (never drawn as a
 * zero), a type the chart has no data for, and a resource with nothing in the
 * range.
 */
export function ProjectMetricsView({
  scope,
  range,
  onRangeChange,
}: ProjectMetricsViewProps) {
  const { t } = useTranslations();
  // One live window for every chart, so their x-axes agree; its tick moves
  // the window, which refetches (no poll timer besides).
  const window = useLiveRange(range);
  const [requestGroupBy, setRequestGroupBy] =
    useState<ProjectMetricGroupBy>("RESOURCE");
  const [percentile, setPercentile] = useState(DEFAULT_PERCENTILE);
  const [hidden, setHidden] = useState<ReadonlySet<string>>(new Set());
  const { currentWorkspace } = useWorkspace();
  const cpu = useProjectMetrics(scope, "cpu", window);
  const memory = useProjectMetrics(scope, "memory", window);
  const instances = useProjectMetrics(scope, "instance_count", window);
  const requests = useProjectMetrics(scope, "http_requests", {
    ...window,
    groupBy: requestGroupBy,
  });
  // Hobby has no response times (Render: Pro and higher): one read of a
  // fixed window (bex-api's default hour) shows the plan notice, where the
  // moving window would only repeat the refusal at every tick.
  const latency = useProjectMetrics(scope, "http_latency", {
    ...(proFeaturesPlan(currentWorkspace?.plan) ? window : {}),
    quantile: Number(percentile),
  });
  const bandwidth = useProjectMetrics(scope, "bandwidth", {
    ...window,
    resolutionSeconds: Math.max(
      MIN_BANDWIDTH_RESOLUTION_SECONDS,
      window.resolutionSeconds,
    ),
  });
  const charts = [cpu, memory, instances, requests, latency, bandwidth];

  // Every chart lists the same readable resources in the same order; the
  // first that answered names them.
  const legend = useMemo(
    () =>
      projectLegend(
        [
          cpu.resources,
          memory.resources,
          instances.resources,
          requests.resources,
          latency.resources,
          bandwidth.resources,
        ].find((resources) => resources.length > 0) ?? [],
      ),
    [
      cpu.resources,
      memory.resources,
      instances.resources,
      requests.resources,
      latency.resources,
      bandwidth.resources,
    ],
  );
  const rangeBound = useMetricsRetentionBound(legend[0]?.id ?? "");
  const tooLarge = charts
    .map((chart) => graphQLErrorExtensions(chart.error, PROJECT_VIEW_TOO_LARGE))
    .find((extensions) => extensions != null);
  // A view with nothing in it: every chart answered, none failed, and none
  // names a resource. Response times are left out: on Hobby they answer the
  // plan's refusal whatever the view holds.
  const empty = [cpu, memory, instances, requests, bandwidth].every(
    (chart) =>
      !chart.loading &&
      !chart.error &&
      !chart.unavailable &&
      !chart.throttled &&
      chart.resources.length === 0 &&
      chart.series.length === 0,
  );

  const toggle = (id: string) =>
    setHidden((previous) => {
      const next = new Set(previous);
      if (!next.delete(id)) next.add(id);
      return next;
    });

  let body: ReactNode;
  if (tooLarge) {
    body = (
      <EmptyState
        role="alert"
        iconName="AlertCircle"
        title={t("metrics.projectViewTooLargeTitle")}
        description={t("metrics.projectViewTooLargeBody", {
          resources: Number(tooLarge["resources"]),
          limit: Number(tooLarge["limit"]),
        })}
      />
    );
  } else if (empty) {
    body = (
      <EmptyState
        iconName="ChartNoAxesCombined"
        title={t("metrics.projectScopeEmptyTitle")}
        description={t("metrics.projectScopeEmptyBody")}
      />
    );
  } else {
    const chart = (
      title: string,
      result: UseProjectMetricsResult,
      groupBy: ProjectMetricGroupBy = "RESOURCE",
      extra?: { headerExtra?: ReactNode; planFeature?: string },
    ) => (
      <ProjectChart
        title={title}
        result={result}
        legend={legend}
        hidden={hidden}
        groupBy={groupBy}
        headerExtra={extra?.headerExtra}
        planFeature={
          extra?.planFeature
            ? {
                feature: extra.planFeature,
                message: t("metrics.latencyPlanRequired"),
              }
            : undefined
        }
      />
    );
    body = (
      <>
        <ProjectLegend legend={legend} hidden={hidden} onToggle={toggle} />
        <Card>
          <CardHeader>
            <CardTitle>{t("metrics.applicationTitle")}</CardTitle>
            <CardDescription>
              {t("metrics.projectApplicationDescription")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {chart(t("metrics.cpu"), cpu)}
            {chart(t("metrics.memory"), memory)}
            {chart(t("metrics.totalInstances"), instances)}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{t("metrics.networkTitle")}</CardTitle>
            <CardDescription>
              {t("metrics.projectNetworkDescription")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {chart(t("metrics.totalRequests"), requests, requestGroupBy, {
              headerExtra: (
                <GroupBySelect
                  value={requestGroupBy}
                  onChange={setRequestGroupBy}
                />
              ),
            })}
            {chart(t("metrics.responseTimes"), latency, "RESOURCE", {
              planFeature: "latencyMetrics",
              headerExtra: (
                <Select value={percentile} onValueChange={setPercentile}>
                  <SelectTrigger
                    size="sm"
                    className="h-7 min-w-36"
                    aria-label={t("metrics.percentile")}
                  >
                    <span className="text-muted-foreground">
                      {t("metrics.percentile")}
                    </span>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {PERCENTILES.map((q) => (
                      <SelectItem key={q.value} value={q.value}>
                        {q.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ),
            })}
            {chart(t("metrics.outboundBandwidth"), bandwidth)}
          </CardContent>
        </Card>
      </>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center gap-3">
        <RangeSelect
          range={range}
          onRangeChange={onRangeChange}
          bound={rangeBound}
        />
      </div>
      {body}
    </div>
  );
}

/**
 * The view's resources, one toggle each in its line's hue: pressed shows the
 * resource on every chart. The row reserves its height before the first
 * answer names them.
 */
function ProjectLegend({
  legend,
  hidden,
  onToggle,
}: {
  legend: readonly ProjectLegendEntry[];
  hidden: ReadonlySet<string>;
  onToggle: (id: string) => void;
}) {
  const { t } = useTranslations();
  return (
    <div
      role="group"
      aria-label={t("metrics.projectLegendLabel")}
      className={PROJECT_LEGEND_ROW_CLASS}
    >
      {legend.map((entry) => {
        const shown = !hidden.has(entry.id);
        return (
          <button
            key={entry.id}
            type="button"
            aria-pressed={shown}
            onClick={() => onToggle(entry.id)}
            className={cn(
              "inline-flex h-7 max-w-full items-center gap-1.5 rounded-md border px-2 text-xs transition-colors",
              shown
                ? "bg-background text-foreground"
                : "border-dashed text-muted-foreground line-through",
            )}
          >
            <span
              aria-hidden="true"
              className={cn(
                "size-2 shrink-0 rounded-full",
                shown ? entry.dotClass : "bg-muted-foreground/40",
              )}
            />
            <span className="truncate">{entry.name}</span>
          </button>
        );
      })}
    </div>
  );
}

/** Total Requests' Group by: Render's metrics-view group-by values. */
function GroupBySelect({
  value,
  onChange,
}: {
  value: ProjectMetricGroupBy;
  onChange: (value: ProjectMetricGroupBy) => void;
}) {
  const { t } = useTranslations();
  return (
    <Select
      value={value}
      onValueChange={(next) => onChange(next as ProjectMetricGroupBy)}
    >
      <SelectTrigger
        size="sm"
        className="h-7 min-w-40"
        aria-label={t("metrics.groupBy")}
      >
        <span className="text-muted-foreground">{t("metrics.groupBy")}</span>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="RESOURCE">{t("metrics.groupByResource")}</SelectItem>
        <SelectItem value="STATUS_CODE">
          {t("metrics.groupByStatus")}
        </SelectItem>
        <SelectItem value="NONE">{t("metrics.groupByAllRequests")}</SelectItem>
      </SelectContent>
    </Select>
  );
}

/** One chart: its section states, its lines, and the notes under it. */
function ProjectChart({
  title,
  result,
  legend,
  hidden,
  groupBy,
  headerExtra,
  planFeature,
}: {
  title: string;
  result: UseProjectMetricsResult;
  legend: readonly ProjectLegendEntry[];
  hidden: ReadonlySet<string>;
  groupBy: ProjectMetricGroupBy;
  headerExtra?: ReactNode;
  planFeature?: { feature: string; message: string };
}) {
  const { t } = useTranslations();
  const lines = useMemo(
    () => projectChartLines(result.series, legend, hidden, "var(--chart-1)"),
    [result.series, legend, hidden],
  );
  const notes = useMemo(
    () => projectChartNotes(result.resources, result.series, groupBy, hidden),
    [result.resources, result.series, groupBy, hidden],
  );
  return (
    <MetricSection
      title={title}
      result={result}
      headerExtra={headerExtra}
      planFeature={planFeature}
    >
      <SvgLineChart unit={result.series[0]?.unit ?? ""} series={lines} />
      {groupBy === "STATUS_CODE" ? <ChartLegend entries={lines} /> : null}
      {notes.failed.length > 0 ? (
        <p
          role="status"
          className="mt-2 flex items-center gap-1.5 text-xs text-destructive"
        >
          <AlertTriangle className="size-3.5 shrink-0" aria-hidden="true" />
          {t("metrics.projectChartFailed", { names: notes.failed.join(", ") })}
        </p>
      ) : null}
      {notes.notCharted.length > 0 ? (
        <p className="mt-1 text-xs text-muted-foreground">
          {t("metrics.projectChartNotCharted", {
            names: notes.notCharted.join(", "),
          })}
        </p>
      ) : null}
      {notes.noData.length > 0 ? (
        <p className="mt-1 text-xs text-muted-foreground">
          {t("metrics.projectChartNoData", { names: notes.noData.join(", ") })}
        </p>
      ) : null}
    </MetricSection>
  );
}
