import {
  RESOURCE_DOT_CLASSES,
  RESOURCE_STROKE_COLORS,
} from "@/features/logs/lib/resource-labels";
import { seriesColor } from "@/features/metrics/components/chart-layout";
import type { ChartSeries } from "@/features/metrics/types";
import type { LineSeriesInput } from "@/features/metrics/components/svg-line-chart";

/**
 * A project's Service Metrics view (w3/m508): what one chart reads, and how
 * its answer becomes lines, a legend and the notes under a chart.
 */

/** The view a chart reads: the project and the view's filter document. */
export interface ProjectMetricsScope {
  project: string;
  filterDocument: Record<string, unknown>;
}

/** Render's metrics-view group-by (its dashboard bundle's enum). */
export type ProjectMetricGroupBy = "RESOURCE" | "STATUS_CODE" | "NONE";

/** What bex-api's read did with one resource of the view. */
export type ProjectMetricResourceStatus = "CHARTED" | "NOT_CHARTED" | "FAILED";

/** One readable resource of the view, as `projectMetrics` lists it. */
export interface ProjectMetricResource {
  id: string;
  name: string;
  /** A service type (web_service, cron_job, …), `postgres` or `key_value`. */
  type: string;
  environmentId: string | null;
  /** A ProjectMetricResourceStatus; a string because the schema's is. */
  status: string;
}

/** A resource as the legend shows it: its name and its hue. */
export interface ProjectLegendEntry {
  id: string;
  name: string;
  /** The line's stroke. */
  color: string;
  /** The legend dot's class, the log views' for the same position. */
  dotClass: string;
}

/**
 * The legend: every readable resource of the view, in the server's order
 * (services by name, then datastores), each with the hue a project log view
 * gives the resource in the same position.
 */
export function projectLegend(
  resources: readonly ProjectMetricResource[],
): ProjectLegendEntry[] {
  return resources.map((r, i) => ({
    id: r.id,
    name: r.name || r.id,
    color: RESOURCE_STROKE_COLORS[i % RESOURCE_STROKE_COLORS.length],
    dotClass: RESOURCE_DOT_CLASSES[i % RESOURCE_DOT_CLASSES.length],
  }));
}

/**
 * A chart's lines. Grouped by resource, each line takes its resource's hue and
 * name, and a resource the legend hides draws none; by status code each code
 * takes a chart hue; ungrouped, the one line takes `soloColor`.
 */
export function projectChartLines(
  series: readonly ChartSeries[],
  legend: readonly ProjectLegendEntry[],
  hidden: ReadonlySet<string>,
  soloColor: string,
): LineSeriesInput[] {
  const byId = new Map(legend.map((entry) => [entry.id, entry]));
  const lines: LineSeriesInput[] = [];
  series.forEach((s, i) => {
    const resource = s.labels["RESOURCE"];
    if (resource !== undefined) {
      if (hidden.has(resource)) return;
      const entry = byId.get(resource);
      lines.push({
        points: s.points,
        color: entry?.color ?? seriesColor(i),
        label: entry?.name ?? resource,
      });
      return;
    }
    const code = s.labels["STATUS_CODE"];
    lines.push(
      code !== undefined
        ? { points: s.points, color: seriesColor(i), label: code }
        : { points: s.points, color: soloColor },
    );
  });
  return lines;
}

/** The names a chart's notes list, by what happened to each resource. */
export interface ProjectChartNotes {
  /** Read and failed: they add nothing to the chart, never a zero. */
  failed: string[];
  /** The chart has no series for their type. */
  notCharted: string[];
  /** Read, with nothing in the range: only when lines are per resource and
   *  others drew (an empty chart already says it has no data). */
  noData: string[];
}

/**
 * Which of the view's resources a chart could not draw, and why. A hidden
 * resource is the reader's own choice and is not listed.
 */
export function projectChartNotes(
  resources: readonly ProjectMetricResource[],
  series: readonly ChartSeries[],
  groupBy: ProjectMetricGroupBy,
  hidden: ReadonlySet<string>,
): ProjectChartNotes {
  const drawn = new Set(
    series.flatMap((s) =>
      s.labels["RESOURCE"] !== undefined ? [s.labels["RESOURCE"]] : [],
    ),
  );
  const notes: ProjectChartNotes = { failed: [], notCharted: [], noData: [] };
  for (const r of resources) {
    if (hidden.has(r.id)) continue;
    const name = r.name || r.id;
    if (r.status === "FAILED") notes.failed.push(name);
    else if (r.status === "NOT_CHARTED") notes.notCharted.push(name);
    else if (groupBy === "RESOURCE" && drawn.size > 0 && !drawn.has(r.id))
      notes.noData.push(name);
  }
  return notes;
}
