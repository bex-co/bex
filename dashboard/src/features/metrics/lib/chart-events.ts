import type { ServiceEventView } from "@/features/events/hooks/use-service-events";

// Chart event markers (Render-parity): service events overlaid on every
// metrics chart as a vertical dashed line + an icon badge in a strip above
// the plot, captured live from Render's dashboard (uPlot `.u-over` overlay:
// one full-height dashed line per event x, one 24px icon per event in a
// 30px strip, nearby events collapsed into a count badge).

/** How a marker renders: Render's start/success/failure/neutral treatments. */
export type ChartEventKind = "start" | "success" | "failure" | "info";

/** The closed set of marker label keys — keeps t() calls type-checked. */
export type ChartEventLabelKey =
  | "metrics.eventDeployStarted"
  | "metrics.eventDeployLiveFor"
  | "metrics.eventDeployLive"
  | "metrics.eventDeployFailed"
  | "metrics.eventDeployCanceled"
  | "metrics.eventDeployEnded"
  | "metrics.eventServerRestarted"
  | "metrics.eventSuspended"
  | "metrics.eventResumed"
  | "metrics.eventInstanceCountChanged"
  | "metrics.eventAutoscalingChanged"
  | "metrics.eventPlanChanged"
  | "metrics.eventsChanged";

export interface ChartEventMarker {
  id: string;
  /** Epoch ms of the event. */
  time: number;
  kind: ChartEventKind;
  /** i18n key for the badge tooltip label (metrics.event*). */
  labelKey: ChartEventLabelKey;
  /** Interpolation params for labelKey (e.g. the commit short id). */
  labelParams?: Record<string, string>;
  /** Humanized event type, shown when labelKey is the generic fallback. */
  fallbackLabel?: string;
  /** Deploy behind this event, when there is one — the badge links to it. */
  deployId?: string;
}

// deploy_ended's wire deployStatus is Render's outcome enum — exactly
// "succeeded" | "canceled" | "failed" (store.RenderDeployStatus). The raw
// internal statuses are accepted too, defensively.
const SUCCEEDED_DEPLOY_STATUSES = new Set(["succeeded", "live"]);
const FAILED_DEPLOY_STATUSES = new Set([
  "failed",
  "build_failed",
  "pre_deploy_failed",
  "update_failed",
]);

/**
 * Maps the service-event feed to chart markers for the current window.
 * Same window semantics as filterTimelineEvents; events without a parseable
 * timestamp never mark a chart.
 */
export function toChartEventMarkers(
  events: ServiceEventView[],
  startTime: string,
  endTime: string,
): ChartEventMarker[] {
  const start = Date.parse(startTime);
  const end = Date.parse(endTime);
  const markers: ChartEventMarker[] = [];
  for (const event of events) {
    const time = Date.parse(event.timestamp ?? "");
    if (!Number.isFinite(time) || time < start || time > end) continue;
    markers.push({
      id: event.id,
      time,
      deployId: event.details?.deployId || undefined,
      ...describeEvent(event),
    });
  }
  return markers.sort((a, b) => a.time - b.time);
}

/** The kind + label for one event — the timeline's taxonomy, Render's styling. */
function describeEvent(
  event: ServiceEventView,
): Pick<
  ChartEventMarker,
  "kind" | "labelKey" | "labelParams" | "fallbackLabel"
> {
  const type = event.type ?? "";
  switch (type) {
    case "deploy_started":
      return { kind: "start", labelKey: "metrics.eventDeployStarted" };
    case "deploy_ended": {
      const status = event.details?.deployStatus ?? "";
      if (SUCCEEDED_DEPLOY_STATUSES.has(status)) {
        const commit = (event.details?.commitId ?? "").slice(0, 7);
        return commit
          ? {
              kind: "success",
              labelKey: "metrics.eventDeployLiveFor",
              labelParams: { commit },
            }
          : { kind: "success", labelKey: "metrics.eventDeployLive" };
      }
      if (FAILED_DEPLOY_STATUSES.has(status)) {
        return { kind: "failure", labelKey: "metrics.eventDeployFailed" };
      }
      if (status === "canceled") {
        return { kind: "info", labelKey: "metrics.eventDeployCanceled" };
      }
      return { kind: "info", labelKey: "metrics.eventDeployEnded" };
    }
    case "server_restarted":
      return { kind: "info", labelKey: "metrics.eventServerRestarted" };
    case "suspender_added":
      return { kind: "info", labelKey: "metrics.eventSuspended" };
    case "suspender_removed":
      return { kind: "info", labelKey: "metrics.eventResumed" };
    case "instance_count_changed":
      return { kind: "info", labelKey: "metrics.eventInstanceCountChanged" };
    case "autoscaling_config_changed":
      return { kind: "info", labelKey: "metrics.eventAutoscalingChanged" };
    case "plan_changed":
      return { kind: "info", labelKey: "metrics.eventPlanChanged" };
    default:
      return {
        kind: "info",
        labelKey: "metrics.eventsChanged",
        fallbackLabel: type ? type.replaceAll("_", " ") : undefined,
      };
  }
}

/** Markers sharing one x position on a chart (Render's "2" count badge). */
export interface MarkerCluster {
  /** viewBox x of the cluster's line + badge. */
  x: number;
  markers: ChartEventMarker[];
}

/**
 * Groups markers whose chart-x positions would overlap into one cluster —
 * Render collapses overlapping event icons into a count badge. xForT is the
 * chart's own time→x mapping so lines land exactly on the plotted samples'
 * scale; x is clamped into [minX, maxX] (an event just past the newest
 * sample still marks the plot edge instead of vanishing).
 */
export function clusterMarkers(
  markers: ChartEventMarker[],
  xForT: (t: number) => number,
  minX: number,
  maxX: number,
  threshold = 20,
): MarkerCluster[] {
  const placed = markers
    .map((m) => ({ m, x: Math.min(maxX, Math.max(minX, xForT(m.time))) }))
    .sort((a, b) => a.x - b.x);
  const clusters: { xs: number[]; markers: ChartEventMarker[] }[] = [];
  for (const { m, x } of placed) {
    const last = clusters[clusters.length - 1];
    if (last && x - last.xs[0] <= threshold) {
      last.xs.push(x);
      last.markers.push(m);
    } else {
      clusters.push({ xs: [x], markers: [m] });
    }
  }
  return clusters.map((c) => ({
    x: c.xs.reduce((sum, x) => sum + x, 0) / c.xs.length,
    markers: c.markers,
  }));
}
