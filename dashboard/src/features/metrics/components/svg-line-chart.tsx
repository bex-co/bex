import { useMemo, useRef, useState } from "react";
import { formatMetricShort } from "@/features/metrics/lib/format";
import {
  CHART_WIDTH as WIDTH,
  CHART_HEIGHT as HEIGHT,
  CHART_PAD as PAD,
} from "@/features/metrics/components/chart-layout";
import type { ChartPoint } from "@/features/metrics/types";

interface SvgLineChartProps {
  points: ChartPoint[];
  unit: string;
  /** A `var(--chart-N)` token — one hue per chart, per the fixed metric->hue mapping. */
  color: string;
}

/**
 * A single-series time-series line: 2px line, ~10% area wash, hairline
 * gridlines, and a crosshair+tooltip that tracks the pointer (or ArrowLeft/
 * ArrowRight when focused) and snaps to the nearest point.
 */
export function SvgLineChart({ points, unit, color }: SvgLineChartProps) {
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  const svgRef = useRef<SVGSVGElement>(null);

  const { path, areaPath, xFor, yFor, yTicks } = useMemo(
    () => buildLineGeometry(points),
    [points],
  );

  if (points.length === 0) {
    return <EmptyChart height={HEIGHT} />;
  }

  const active = activeIndex != null ? points[activeIndex] : null;

  function nearestIndexForClientX(clientX: number): number {
    const svg = svgRef.current;
    if (!svg) return 0;
    const rect = svg.getBoundingClientRect();
    const relX = ((clientX - rect.left) / rect.width) * WIDTH;
    let best = 0;
    let bestDist = Infinity;
    points.forEach((_, i) => {
      const d = Math.abs(xFor(i) - relX);
      if (d < bestDist) {
        bestDist = d;
        best = i;
      }
    });
    return best;
  }

  return (
    <div className="relative">
      <svg
        ref={svgRef}
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        className="w-full"
        role="img"
        aria-label={`Line chart with ${points.length} data points`}
        tabIndex={0}
        onPointerMove={(e) => setActiveIndex(nearestIndexForClientX(e.clientX))}
        onPointerLeave={() => setActiveIndex(null)}
        onKeyDown={(e) => {
          if (e.key === "ArrowRight") {
            setActiveIndex((i) => Math.min(points.length - 1, (i ?? -1) + 1));
          } else if (e.key === "ArrowLeft") {
            setActiveIndex((i) => Math.max(0, (i ?? points.length) - 1));
          } else if (e.key === "Escape") {
            setActiveIndex(null);
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

        {/* Area wash (~10% opacity) then the 2px line, in that fixed order. */}
        <path d={areaPath} fill={color} opacity={0.1} stroke="none" />
        <path
          d={path}
          fill="none"
          stroke={color}
          strokeWidth={2}
          strokeLinejoin="round"
          strokeLinecap="round"
        />

        {/* Crosshair + active marker. */}
        {active && activeIndex != null && (
          <g>
            <line
              x1={xFor(activeIndex)}
              x2={xFor(activeIndex)}
              y1={PAD.top}
              y2={HEIGHT - PAD.bottom}
              stroke="var(--border)"
              strokeWidth={1}
            />
            <circle
              cx={xFor(activeIndex)}
              cy={yFor(active.value)}
              r={4}
              fill={color}
              stroke="var(--background)"
              strokeWidth={2}
            />
          </g>
        )}
      </svg>

      {active && (
        <ChartTooltip
          label={new Date(active.timestamp).toLocaleTimeString()}
          value={formatMetricShort(unit, active.value)}
          color={color}
        />
      )}
    </div>
  );
}

export function ChartTooltip({
  label,
  value,
  color,
}: {
  label: string;
  value: string;
  color: string;
}) {
  return (
    <div className="pointer-events-none absolute top-1 right-1 rounded-md border bg-popover px-2 py-1 text-xs shadow-sm">
      <div className="flex items-center gap-1.5">
        <span
          className="inline-block h-0.5 w-3 rounded-full"
          style={{ backgroundColor: color }}
        />
        <span className="font-semibold text-popover-foreground">{value}</span>
      </div>
      <div className="text-muted-foreground">{label}</div>
    </div>
  );
}

export function EmptyChart({ height = HEIGHT }: { height?: number }) {
  return (
    <div
      className="flex items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground"
      style={{ height }}
    >
      No data in range
    </div>
  );
}

function buildLineGeometry(points: ChartPoint[]) {
  const values = points.map((p) => p.value);
  const maxVal = Math.max(...values, 0);
  const minVal = Math.min(...values, 0);
  const span = maxVal - minVal || 1;

  const innerWidth = WIDTH - PAD.left - PAD.right;
  const innerHeight = HEIGHT - PAD.top - PAD.bottom;

  const xFor = (i: number) =>
    points.length === 1
      ? PAD.left + innerWidth / 2
      : PAD.left + (i / (points.length - 1)) * innerWidth;
  const yFor = (v: number) =>
    PAD.top + innerHeight - ((v - minVal) / span) * innerHeight;

  const line = points
    .map((p, i) => `${i === 0 ? "M" : "L"}${xFor(i)},${yFor(p.value)}`)
    .join(" ");
  const area =
    points.length > 0
      ? `${line} L${xFor(points.length - 1)},${PAD.top + innerHeight} L${xFor(0)},${PAD.top + innerHeight} Z`
      : "";

  // Round ticks: min, midpoint, max — enough for a compact PoC axis.
  const yTicks = [minVal, (minVal + maxVal) / 2, maxVal].map((v) => ({
    value: v,
    y: yFor(v),
  }));

  return { path: line, areaPath: area, xFor, yFor, yTicks };
}
