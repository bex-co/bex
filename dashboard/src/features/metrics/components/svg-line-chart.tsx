import { useMemo, useRef, useState } from "react";
import { formatMetricShort } from "@/features/metrics/lib/format";
import {
  CHART_WIDTH as WIDTH,
  CHART_HEIGHT as HEIGHT,
  CHART_PAD as PAD,
  groupPointsByTime,
  frameTooltipRows,
  type SeriesInput,
  type ChartFrame,
} from "@/features/metrics/components/chart-layout";

/** One line of a (possibly multi-series) chart — e.g. one App instance. */
export type LineSeriesInput = SeriesInput;

interface SvgLineChartProps {
  unit: string;
  /** One line per entry (e.g. one per instance); one entry = a plain chart. */
  series: LineSeriesInput[];
  /** Dashed horizontal reference (e.g. a resource limit), drawn when within ~2x the data max. */
  referenceValue?: number;
}

/**
 * A time-series line chart: 2px lines, hairline gridlines, and a crosshair+
 * tooltip that tracks the pointer (or ArrowLeft/ArrowRight when focused) and
 * snaps to the nearest sample time. Accepts one or many series; x is mapped by
 * timestamp (not index) so series that start late — a pod created mid-range —
 * still align. The area wash only draws for a single series (stacked washes
 * turn to mud).
 */
export function SvgLineChart({
  unit,
  series,
  referenceValue,
}: SvgLineChartProps) {
  const [activeFrame, setActiveFrame] = useState<number | null>(null);
  const svgRef = useRef<SVGSVGElement>(null);

  const geo = useMemo(
    () => buildGeometry(series, referenceValue),
    [series, referenceValue],
  );

  if (!geo) {
    return <EmptyChart height={HEIGHT} />;
  }

  const { frames, xForT, yFor, yTicks, paths, referenceY } = geo;

  function nearestFrameForClientX(clientX: number): number {
    const svg = svgRef.current;
    if (!svg || !geo) return 0;
    const rect = svg.getBoundingClientRect();
    const relX = ((clientX - rect.left) / rect.width) * WIDTH;
    let best = 0;
    let bestDist = Infinity;
    geo.frames.forEach((f, i) => {
      const d = Math.abs(geo.xForT(f.time) - relX);
      if (d < bestDist) {
        bestDist = d;
        best = i;
      }
    });
    return best;
  }

  const active = activeFrame != null ? frames[activeFrame] : null;

  return (
    <div className="relative">
      <svg
        ref={svgRef}
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        className="w-full"
        role="img"
        aria-label={`Line chart with ${frames.length} data points`}
        tabIndex={0}
        onPointerMove={(e) => setActiveFrame(nearestFrameForClientX(e.clientX))}
        onPointerLeave={() => setActiveFrame(null)}
        onKeyDown={(e) => {
          if (e.key === "ArrowRight") {
            setActiveFrame((i) => Math.min(frames.length - 1, (i ?? -1) + 1));
          } else if (e.key === "ArrowLeft") {
            setActiveFrame((i) => Math.max(0, (i ?? frames.length) - 1));
          } else if (e.key === "Escape") {
            setActiveFrame(null);
          }
        }}
      >
        {/* Gridlines: hairline, recessive, one step off the surface. Keyed by
            position, not value — a flat series (all zero) repeats the same
            value across every tick. */}
        {yTicks.map((t, i) => (
          <g key={i}>
            <line
              x1={PAD.left}
              x2={WIDTH - PAD.right}
              y1={t.y}
              y2={t.y}
              stroke="var(--border)"
              strokeWidth={1}
            />
            <text
              x={PAD.left - 6}
              y={t.y}
              textAnchor="end"
              dominantBaseline="middle"
              className="fill-muted-foreground text-[9px]"
            >
              {formatMetricShort(unit, t.value)}
            </text>
          </g>
        ))}

        {/* Reference (e.g. limit): dashed, recessive, above gridlines. */}
        {referenceY != null && (
          <line
            x1={PAD.left}
            x2={WIDTH - PAD.right}
            y1={referenceY}
            y2={referenceY}
            stroke="var(--muted-foreground)"
            strokeWidth={1}
            strokeDasharray="4 3"
          />
        )}

        {/* Area wash (~10% opacity, single series only) then the 2px lines. */}
        {paths.map((p, i) => (
          <g key={i}>
            {p.areaPath && (
              <path d={p.areaPath} fill={p.color} opacity={0.1} stroke="none" />
            )}
            <path
              d={p.linePath}
              fill="none"
              stroke={p.color}
              strokeWidth={2}
              strokeLinejoin="round"
              strokeLinecap="round"
            />
          </g>
        ))}

        {/* Crosshair + active markers, one per series with a sample here. */}
        {active && (
          <g>
            <line
              x1={xForT(active.time)}
              x2={xForT(active.time)}
              y1={PAD.top}
              y2={HEIGHT - PAD.bottom}
              stroke="var(--border)"
              strokeWidth={1}
            />
            {active.rows.map((row, i) => (
              <circle
                key={i}
                cx={xForT(active.time)}
                cy={yFor(row.value)}
                r={4}
                fill={row.color}
                stroke="var(--background)"
                strokeWidth={2}
              />
            ))}
          </g>
        )}
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

export interface TooltipRow {
  color: string;
  value: string;
  /** Series name; omitted on single-series charts. */
  name?: string;
}

export function ChartTooltip({
  label,
  rows,
}: {
  label: string;
  rows: TooltipRow[];
}) {
  return (
    <div className="pointer-events-none absolute top-1 right-1 rounded-md border bg-popover px-2 py-1 text-xs shadow-sm">
      {rows.map((row, i) => (
        <div key={i} className="flex items-center gap-1.5">
          <span
            className="inline-block h-0.5 w-3 rounded-full"
            style={{ backgroundColor: row.color }}
          />
          {row.name && (
            <span className="max-w-40 truncate text-muted-foreground">
              {row.name}
            </span>
          )}
          <span className="font-semibold text-popover-foreground">
            {row.value}
          </span>
        </div>
      ))}
      <div className="text-muted-foreground">{label}</div>
    </div>
  );
}

export function EmptyChart({
  height = HEIGHT,
  message = "No data in range",
}: {
  height?: number;
  message?: string;
}) {
  return (
    <div
      className="flex items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground"
      style={{ height }}
    >
      {message}
    </div>
  );
}

/**
 * A legend row for a multi-series chart: one colored dot + label per named
 * series. Hides itself for single-series (or unlabeled) charts.
 */
export function ChartLegend({
  entries,
}: {
  entries: { color: string; label?: string }[];
}) {
  const named = entries.filter(
    (e): e is { color: string; label: string } => !!e.label,
  );
  if (named.length < 2) return null;
  return (
    <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1">
      {named.map((e, i) => (
        <span
          key={i}
          className="flex items-center gap-1.5 text-xs text-muted-foreground"
        >
          <span
            className="inline-block h-2 w-2 rounded-full"
            style={{ backgroundColor: e.color }}
          />
          {e.label}
        </span>
      ))}
    </div>
  );
}

interface LineGeometry {
  frames: ChartFrame[];
  xForT: (t: number) => number;
  yFor: (v: number) => number;
  yTicks: { value: number; y: number }[];
  paths: { linePath: string; areaPath: string; color: string }[];
  /** y of the reference line, when one is drawn. */
  referenceY: number | null;
}

/**
 * Time-mapped geometry shared by every series: x from the union time domain,
 * y from the union value domain (plus the reference value when it's close
 * enough to the data to draw without flattening it). Null when there is
 * nothing to draw.
 */
function buildGeometry(
  seriesList: LineSeriesInput[],
  referenceValue: number | undefined,
): LineGeometry | null {
  const drawn = seriesList.filter((s) => s.points.length > 0);
  const allPoints = drawn.flatMap((s) => s.points);
  if (allPoints.length === 0) {
    return null;
  }

  const times = allPoints.map((p) => Date.parse(p.timestamp));
  const minT = Math.min(...times);
  const maxT = Math.max(...times);
  const values = allPoints.map((p) => p.value);
  const dataMax = Math.max(...values, 0);
  const minVal = Math.min(...values, 0);
  // Include a nearby reference (limit) in the domain; a far-away one (usage
  // at 1% of limit) would flatten the data to a hairline, so it's dropped.
  const showReference =
    referenceValue != null && referenceValue <= dataMax * 2.5;
  const maxVal =
    showReference && referenceValue != null
      ? Math.max(dataMax, referenceValue)
      : dataMax;
  const span = maxVal - minVal || 1;

  const innerWidth = WIDTH - PAD.left - PAD.right;
  const innerHeight = HEIGHT - PAD.top - PAD.bottom;

  const xForT = (t: number) =>
    maxT === minT
      ? PAD.left + innerWidth / 2
      : PAD.left + ((t - minT) / (maxT - minT)) * innerWidth;
  const yFor = (v: number) =>
    PAD.top + innerHeight - ((v - minVal) / span) * innerHeight;

  const paths = drawn.map((s) => {
    const line = s.points
      .map(
        (p, i) =>
          `${i === 0 ? "M" : "L"}${xForT(Date.parse(p.timestamp))},${yFor(p.value)}`,
      )
      .join(" ");
    const first = s.points[0];
    const last = s.points[s.points.length - 1];
    const area =
      drawn.length === 1
        ? `${line} L${xForT(Date.parse(last.timestamp))},${PAD.top + innerHeight} L${xForT(Date.parse(first.timestamp))},${PAD.top + innerHeight} Z`
        : "";
    return { linePath: line, areaPath: area, color: s.color };
  });

  // Round ticks: min, midpoint, max — enough for a compact PoC axis.
  const yTicks = [minVal, (minVal + maxVal) / 2, maxVal].map((v) => ({
    value: v,
    y: yFor(v),
  }));

  return {
    frames: groupPointsByTime(drawn),
    xForT,
    yFor,
    yTicks,
    paths,
    referenceY:
      showReference && referenceValue != null ? yFor(referenceValue) : null,
  };
}
