import { formatMetricShort } from "@/features/metrics/lib/format";
import type { ChartPoint } from "@/features/metrics/types";
import type { TooltipRow } from "@/features/metrics/components/svg-line-chart";

// Shared SVG canvas dimensions for SvgLineChart and SvgBarChart — the two
// render side-by-side in the same card, so keeping these in one place (rather
// than duplicated per file) is what keeps them the same size.
export const CHART_WIDTH = 600;
export const CHART_HEIGHT = 180;
export const CHART_PAD = { top: 8, right: 8, bottom: 20, left: 44 };

/** One series of a (possibly multi-series) chart — e.g. one instance or group. */
export interface SeriesInput {
  points: ChartPoint[];
  /** A `var(--chart-N)` token — one hue per series. */
  color: string;
  /** Tooltip/legend name; omitted for single-series charts. */
  label?: string;
}

// The fixed hue ladder multi-series charts cycle through.
export const CHART_SERIES_COLORS = [
  "var(--chart-1)",
  "var(--chart-2)",
  "var(--chart-3)",
  "var(--chart-4)",
  "var(--chart-5)",
];

/** The i-th series' hue; startAt offsets the ladder (a chart's base hue). */
export function seriesColor(i: number, startAt = 0): string {
  return CHART_SERIES_COLORS[(startAt + i) % CHART_SERIES_COLORS.length];
}

/** One frame per distinct sample time: every series' value observed then. */
export interface ChartFrame {
  time: number;
  /** Sum across rows — the stacked-bar height; single-series = the value. */
  total: number;
  rows: { value: number; color: string; label?: string }[];
}

/**
 * Buckets every series' points by sample time (the union across series) —
 * the shared tooltip/stacking mechanism of both charts. Series share a query
 * step, so times align; a late-starting series (a pod created mid-range)
 * simply has no row in earlier frames.
 */
export function groupPointsByTime(seriesList: SeriesInput[]): ChartFrame[] {
  const byTime = new Map<number, ChartFrame>();
  for (const s of seriesList) {
    for (const p of s.points) {
      const t = Date.parse(p.timestamp);
      let frame = byTime.get(t);
      if (!frame) {
        frame = { time: t, total: 0, rows: [] };
        byTime.set(t, frame);
      }
      frame.rows.push({ value: p.value, color: s.color, label: s.label });
      frame.total += p.value;
    }
  }
  return [...byTime.values()].sort((a, b) => a.time - b.time);
}

/** A frame's rows as tooltip rows, values formatted for the chart's unit. */
export function frameTooltipRows(
  frame: ChartFrame,
  unit: string,
): TooltipRow[] {
  return frame.rows.map((r) => ({
    color: r.color,
    value: formatMetricShort(unit, r.value),
    name: r.label,
  }));
}
