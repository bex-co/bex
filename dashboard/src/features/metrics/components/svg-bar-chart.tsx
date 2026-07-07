import { useState } from "react";
import { formatMetricShort } from "@/features/metrics/lib/format";
import {
  ChartTooltip,
  EmptyChart,
} from "@/features/metrics/components/svg-line-chart";
import type { ChartPoint } from "@/features/metrics/types";

const WIDTH = 600;
const HEIGHT = 180;
const PAD = { top: 8, right: 8, bottom: 20, left: 44 };
const MAX_BAR_WIDTH = 24;
const BAR_GAP = 2;

interface SvgBarChartProps {
  points: ChartPoint[];
  unit: string;
  color: string;
}

/**
 * A single-series bar chart: bars capped at 24px with a 4px rounded top and
 * square baseline, a 2px surface gap between neighbors, hairline gridlines,
 * and a per-bar hover/focus tooltip (the bar itself is the hit target — no
 * crosshair, per dataviz's bar-vs-line interaction split).
 */
export function SvgBarChart({ points, unit, color }: SvgBarChartProps) {
  const [activeIndex, setActiveIndex] = useState<number | null>(null);

  if (points.length === 0) {
    return <EmptyChart height={HEIGHT} />;
  }

  const values = points.map((p) => p.value);
  const maxVal = Math.max(...values, 0);
  const innerWidth = WIDTH - PAD.left - PAD.right;
  const innerHeight = HEIGHT - PAD.top - PAD.bottom;
  const baseline = PAD.top + innerHeight;

  const slot = innerWidth / points.length;
  const barWidth = Math.max(1, Math.min(MAX_BAR_WIDTH, slot - BAR_GAP));

  const yTicks = [0, maxVal / 2, maxVal];
  const active = activeIndex != null ? points[activeIndex] : null;

  return (
    <div className="relative">
      <svg
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        className="w-full"
        role="img"
        aria-label={`Bar chart with ${points.length} data points`}
      >
        {yTicks.map((v) => {
          const y = baseline - (maxVal === 0 ? 0 : (v / maxVal) * innerHeight);
          return (
            <g key={v}>
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

        {points.map((p, i) => {
          const h = maxVal === 0 ? 0 : (p.value / maxVal) * innerHeight;
          const x = PAD.left + i * slot + (slot - barWidth) / 2;
          const y = baseline - h;
          const isActive = activeIndex === i;
          return (
            <path
              key={p.timestamp + i}
              d={roundedTopBarPath(x, y, barWidth, h, 4)}
              fill={color}
              opacity={isActive ? 1 : 0.85}
              tabIndex={0}
              role="graphics-symbol"
              aria-label={`${formatMetricShort(unit, p.value)} at ${new Date(p.timestamp).toLocaleTimeString()}`}
              onPointerEnter={() => setActiveIndex(i)}
              onFocus={() => setActiveIndex(i)}
              onPointerLeave={() =>
                setActiveIndex((cur) => (cur === i ? null : cur))
              }
              onBlur={() => setActiveIndex((cur) => (cur === i ? null : cur))}
            />
          );
        })}
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
