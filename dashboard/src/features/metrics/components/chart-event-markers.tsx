import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { CircleDot, Rocket } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServiceBase } from "@/features/services/lib/service-base";
import {
  CHART_WIDTH as WIDTH,
  CHART_HEIGHT as HEIGHT,
  CHART_PAD as PAD,
} from "@/features/metrics/components/chart-layout";
import type {
  ChartEventKind,
  ChartEventMarker,
  MarkerCluster,
} from "@/features/metrics/lib/chart-events";

// The two halves of a chart's event-marker overlay (Render parity, captured
// live from dashboard.render.com): ChartEventLines draws a full-height dashed
// vertical line per cluster inside the SVG; ChartEventStrip renders the icon
// badges in an HTML strip above it (HTML so the badges can be links and carry
// a hover tooltip). Both position by the same viewBox x — the strip converts
// it to a percentage, which lines up because the SVG spans the full width.

/** Vertical dashed line per event cluster — rendered inside the chart SVG. */
export function ChartEventLines({ clusters }: { clusters: MarkerCluster[] }) {
  return (
    <g>
      {clusters.map((c) => (
        <line
          key={c.x}
          x1={c.x}
          x2={c.x}
          y1={PAD.top}
          y2={HEIGHT - PAD.bottom}
          stroke="var(--muted-foreground)"
          strokeWidth={1}
          strokeDasharray="3 3"
          opacity={0.6}
        />
      ))}
    </g>
  );
}

const KIND_BADGE_CLASS: Record<ChartEventKind, string> = {
  start:
    "border-dashed border-muted-foreground/60 bg-background text-muted-foreground",
  success:
    "border-emerald-600/60 bg-emerald-500/15 text-emerald-600 dark:text-emerald-400",
  failure: "border-destructive/60 bg-destructive/10 text-destructive",
  info: "border-border bg-muted text-muted-foreground",
};

const DEPLOY_KINDS = new Set<ChartEventKind>(["start", "success", "failure"]);

/**
 * The icon-badge strip above a chart: one badge per cluster, a count badge
 * when events collapse together. Hover shows the events' labels + times;
 * click goes to the deploy (single deploy event) or the Events tab.
 */
export function ChartEventStrip({
  clusters,
  serviceId,
}: {
  clusters: MarkerCluster[];
  serviceId: string;
}) {
  const { t } = useTranslations();
  const base = useServiceBase();
  const [active, setActive] = useState<number | null>(null);

  if (clusters.length === 0) return null;

  return (
    <div
      className="relative h-6"
      role="list"
      aria-label={t("metrics.eventMarkersLabel")}
    >
      {clusters.map((cluster, i) => {
        const single = cluster.markers.length === 1 ? cluster.markers[0] : null;
        const badge = (
          <span
            className={`flex h-5 w-5 items-center justify-center rounded-sm border text-[10px] font-medium ${badgeClass(cluster)}`}
          >
            {single ? <BadgeIcon kind={single.kind} /> : cluster.markers.length}
          </span>
        );
        return (
          <div
            key={cluster.x}
            role="listitem"
            className="absolute top-0 -translate-x-1/2"
            style={{ left: `${(cluster.x / WIDTH) * 100}%` }}
            onPointerEnter={() => setActive(i)}
            onPointerLeave={() => setActive((cur) => (cur === i ? null : cur))}
          >
            {single?.deployId ? (
              <Link
                to={`${base}/$serviceId/deploys/$deployId`}
                params={{ serviceId, deployId: single.deployId }}
                aria-label={markerLabel(single, t)}
                className="block rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                onFocus={() => setActive(i)}
                onBlur={() => setActive((cur) => (cur === i ? null : cur))}
              >
                {badge}
              </Link>
            ) : (
              <Link
                to={`${base}/$serviceId/events`}
                params={{ serviceId }}
                aria-label={
                  single
                    ? markerLabel(single, t)
                    : t("metrics.eventMarkerCluster", {
                        count: String(cluster.markers.length),
                      })
                }
                className="block rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                onFocus={() => setActive(i)}
                onBlur={() => setActive((cur) => (cur === i ? null : cur))}
              >
                {badge}
              </Link>
            )}
            {active === i && (
              <div className="pointer-events-none absolute bottom-full left-1/2 z-10 mb-1 w-max max-w-64 -translate-x-1/2 rounded-md border bg-popover px-2 py-1.5 text-xs shadow-sm">
                {cluster.markers.map((marker) => (
                  <div key={marker.id} className="flex items-center gap-1.5">
                    <span className={tooltipDotClass(marker.kind)} />
                    <span className="truncate font-medium text-popover-foreground">
                      {markerLabel(marker, t)}
                    </span>
                    <span className="shrink-0 text-muted-foreground">
                      {new Date(marker.time).toLocaleTimeString()}
                    </span>
                  </div>
                ))}
                <div className="mt-0.5 text-muted-foreground">
                  {t("metrics.eventMarkerHint")}
                </div>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function BadgeIcon({ kind }: { kind: ChartEventKind }) {
  return DEPLOY_KINDS.has(kind) ? (
    <Rocket className="size-3" />
  ) : (
    <CircleDot className="size-3" />
  );
}

/** A cluster badge inherits its most severe member's treatment. */
function badgeClass(cluster: MarkerCluster): string {
  const kinds = new Set(cluster.markers.map((m) => m.kind));
  if (kinds.has("failure")) return KIND_BADGE_CLASS.failure;
  if (kinds.has("success")) return KIND_BADGE_CLASS.success;
  if (kinds.has("start")) return KIND_BADGE_CLASS.start;
  return KIND_BADGE_CLASS.info;
}

const TOOLTIP_DOT: Record<ChartEventKind, string> = {
  start: "bg-muted-foreground",
  success: "bg-emerald-500",
  failure: "bg-destructive",
  info: "bg-muted-foreground",
};

function tooltipDotClass(kind: ChartEventKind): string {
  return `inline-block size-1.5 shrink-0 rounded-full ${TOOLTIP_DOT[kind]}`;
}

function markerLabel(
  marker: ChartEventMarker,
  t: (
    key: ChartEventMarker["labelKey"],
    params?: Record<string, string>,
  ) => string,
): string {
  if (marker.fallbackLabel) return marker.fallbackLabel;
  return t(marker.labelKey, marker.labelParams);
}
