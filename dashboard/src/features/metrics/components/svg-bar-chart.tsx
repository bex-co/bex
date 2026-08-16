import { useMemo, useState } from "react";
import { formatMetricShort } from "@/features/metrics/lib/format";
import {
  ChartTooltip,
  EmptyChart,
} from "@/features/metrics/components/svg-line-chart";
import {
  CHART_WIDTH as WIDTH,
  CHART_HEIGHT as HEIGHT,
  CHART_PAD as PAD,
  groupPointsByTime,
  frameTooltipRows,
  type SeriesInput,
} from "@/features/metrics/components/chart-layout";
import {
  ChartEventLines,
  ChartEventStrip,
} from "@/features/metrics/components/chart-event-markers";
import {
  clusterMarkers,
  type ChartEventMarker,
} from "@/features/metrics/lib/chart-events";

const MAX_BAR_WIDTH = 24;
const BAR_GAP = 2;

/** One stack segment source of a (possibly grouped) bar chart. */
export type BarSeriesInput = SeriesInput;

interface SvgBarChartProps {
  unit: string;
  /** Segments stack per time bucket (e.g. per status code); one entry = plain bars. */
  series: BarSeriesInput[];
  /** Service events to mark on the chart (vertical line + badge strip). */
  markers?: ChartEventMarker[];
  /** The service the markers' badges link into; required to render markers. */
  markersServiceId?: string;
}

/**
 * A bar chart: bars capped at 24px with a 4px rounded top and square baseline,
 * a 2px surface gap between neighbors, hairline gridlines, and a per-bar
 * hover/focus tooltip (the bar itself is the hit target — no crosshair, per
 * dataviz's bar-vs-line interaction split). With multiple series the segments
 * stack per time bucket (Render's grouped Total Requests), sharing one y scale
 * normalized to the tallest stack.
 */
export function SvgBarChart({
  unit,
  series,
  markers,
  markersServiceId,
}: SvgBarChartProps) {
  const [activeIndex, setActiveIndex] = useState<number | null>(null);

  const frames = useMemo(() => groupPointsByTime(series), [series]);

  // Bars are slot-positioned by index, so the marker x-mapping interpolates a
  // fractional index from the frame times (frames share one query step, so
  // index is linear in time) and lands on that slot's center.
  const eventClusters = useMemo(() => {
    if (!markers?.length || !markersServiceId || frames.length === 0) {
      return [];
    }
    const innerWidth = WIDTH - PAD.left - PAD.right;
    const slot = innerWidth / frames.length;
    const t0 = frames[0].time;
    const tN = frames[frames.length - 1].time;
    const xForT = (t: number) =>
      tN === t0
        ? PAD.left + innerWidth / 2
        : PAD.left +
          (((t - t0) / (tN - t0)) * (frames.length - 1) + 0.5) * slot;
    return clusterMarkers(markers, xForT, PAD.left, WIDTH - PAD.right);
  }, [markers, markersServiceId, frames]);

  if (frames.length === 0) {
    return <EmptyChart height={HEIGHT} />;
  }

  const maxVal = Math.max(...frames.map((f) => f.total), 0);
  const innerWidth = WIDTH - PAD.left - PAD.right;
  const innerHeight = HEIGHT - PAD.top - PAD.bottom;
  const baseline = PAD.top + innerHeight;

  const slot = innerWidth / frames.length;
  const barWidth = Math.max(1, Math.min(MAX_BAR_WIDTH, slot - BAR_GAP));

  const yTicks = [0, maxVal / 2, maxVal];
  const active = activeIndex != null ? frames[activeIndex] : null;

  return (
    <div className="relative">
      {markersServiceId && (
        <ChartEventStrip
          clusters={eventClusters}
          serviceId={markersServiceId}
        />
      )}
      <svg
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        className="w-full"
        role="img"
        aria-label={`Bar chart with ${frames.length} data points`}
      >
        {/* Keyed by position, not value — an all-zero series repeats 0 at every tick. */}
        {yTicks.map((v, i) => {
          const y = baseline - (maxVal === 0 ? 0 : (v / maxVal) * innerHeight);
          return (
            <g key={i}>
              <line
                x1={PAD.left}
                x2={WIDTH - PAD.right}
                y1={y}
                y2={y}
                stroke="var(--border)"
                strokeWidth={1}
              />
              <text
                x={PAD.left - 6}
                y={y}
                textAnchor="end"
                dominantBaseline="middle"
                className="fill-muted-foreground text-[9px]"
              >
                {formatMetricShort(unit, v)}
              </text>
            </g>
          );
        })}

        {/* Event markers' vertical lines, under the bars. */}
        {eventClusters.length > 0 && (
          <ChartEventLines clusters={eventClusters} />
        )}

        {frames.map((frame, i) => {
          const x = PAD.left + i * slot + (slot - barWidth) / 2;
          const isActive = activeIndex === i;
          const stackH =
            maxVal === 0 ? 0 : (frame.total / maxVal) * innerHeight;
          let yCursor = baseline;
          return (
            <g
              key={frame.time + i}
              tabIndex={0}
              role="graphics-symbol"
              aria-label={`${formatMetricShort(unit, frame.total)} at ${new Date(frame.time).toLocaleTimeString()}`}
              onPointerEnter={() => setActiveIndex(i)}
              onFocus={() => setActiveIndex(i)}
              onPointerLeave={() =>
                setActiveIndex((cur) => (cur === i ? null : cur))
              }
              onBlur={() => setActiveIndex((cur) => (cur === i ? null : cur))}
            >
              {frame.rows.map((row, j) => {
                const h = maxVal === 0 ? 0 : (row.value / maxVal) * innerHeight;
                yCursor -= h;
                const isTop = yCursor - (baseline - stackH) < 0.5;
                return (
                  <path
                    key={j}
                    d={roundedTopBarPath(
                      x,
                      yCursor,
                      barWidth,
                      h,
                      isTop ? 4 : 0,
                    )}
                    fill={row.color}
                    opacity={isActive ? 1 : 0.85}
                  />
                );
              })}
            </g>
          );
        })}
      </svg>

      {active && (
        <ChartTooltip
          label={new Date(active.time).toLocaleTimeString()}
          rows={frameTooltipRows(active, unit)}
        />
      )}
    </div>
  );
}

/** A bar path with rounded top corners and a square baseline (data-end vs. baseline). */
function roundedTopBarPath(
  x: number,
  y: number,
  width: number,
  height: number,
  radius: number,
): string {
  const r = Math.min(radius, width / 2, height);
  if (height <= 0 || r <= 0) {
    return `M${x},${y + height} h${width} v${-height} h${-width} Z`;
  }
  return [
    `M${x},${y + height}`,
    `V${y + r}`,
    `Q${x},${y} ${x + r},${y}`,
    `H${x + width - r}`,
    `Q${x + width},${y} ${x + width},${y + r}`,
    `V${y + height}`,
    "Z",
  ].join(" ");
}
