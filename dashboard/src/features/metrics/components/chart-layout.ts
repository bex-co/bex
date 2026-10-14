import { formatMetricShort } from "@/features/metrics/lib/format";
import type { ChartPoint } from "@/features/metrics/types";
import type { TooltipRow } from "@/features/metrics/components/svg-line-chart";

// Shared SVG canvas dimensions for SvgLineChart and SvgBarChart — the two
// render side-by-side in the same card, so keeping these in one place (rather
// than duplicated per file) is what keeps them the same size.
export const CHART_WIDTH = 600;
export const CHART_HEIGHT = 180;
export const CHART_PAD = { top: 8, right: 8, bottom: 20, left: 44 };

/**
 * The populated chart is a width-100% SVG with a CHART_WIDTH×CHART_HEIGHT
 * viewBox, so it scales to that ratio (256px tall at 854px, 95px at 316px).
 * Every non-data frame — loading, empty, unavailable, error — takes the same
 * ratio, so a section never changes height when its data arrives (w4/m209).
 */
export const CHART_FRAME_STYLE = {
  aspectRatio: `${CHART_WIDTH} / ${CHART_HEIGHT}`,
} as const;

// Tick labels are end-anchored TICK_LABEL_GAP left of the plot, at the
// 9px axis font, and keep TICK_INSET clear of the canvas edge.
const TICK_FONT_PX = 9;
const TICK_LABEL_GAP = 6;
const TICK_INSET = 4;
let measureCtx: CanvasRenderingContext2D | null | undefined;

/** A tick label's width at the axis font, in viewBox units. */
export function tickLabelWidth(label: string): number {
  if (measureCtx === undefined && typeof document !== "undefined") {
    try {
      measureCtx = document.createElement("canvas").getContext("2d");
      if (measureCtx) {
        const family = getComputedStyle(document.body).fontFamily || "sans-serif";
        measureCtx.font = `${TICK_FONT_PX}px ${family}`;
      }
    } catch {
      measureCtx = null;
    }
  }
  const measured = measureCtx?.measureText(label).width;
  // Without a canvas, a bounded estimate: 0.6em per character.
  return measured && measured > 0 ? measured : label.length * TICK_FONT_PX * 0.6;
}

/**
 * The plot's left gutter for these formatted y-tick labels: wide enough for
 * the longest at the axis font plus its gap and inset, never narrower than
 * CHART_PAD.left (w4/m209 — "536.7 KiB" no longer starts before the canvas).
 */
export function axisLeftPad(labels: string[]): number {
  const widest = Math.max(0, ...labels.map(tickLabelWidth));
  return Math.max(CHART_PAD.left, Math.ceil(widest + TICK_LABEL_GAP + TICK_INSET));
}

/** The x of an end-anchored tick label for a plot starting at left. */
export function tickLabelX(left: number): number {
  return left - TICK_LABEL_GAP;
}

/** A frame's style: an explicit caller height, else the chart's ratio. */
export function chartFrameStyle(height?: number) {
  return height != null ? { height } : CHART_FRAME_STYLE;
}

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
