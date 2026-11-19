import { createContext, useContext, useLayoutEffect, useState } from "react";
import { formatMetricShort } from "@/features/metrics/lib/format";
import type { ChartPoint } from "@/features/metrics/types";
import type { TooltipRow } from "@/features/metrics/components/svg-line-chart";

// Shared SVG canvas dimensions for SvgLineChart and SvgBarChart — the two
// render side-by-side in the same card, so keeping these in one place (rather
// than duplicated per file) is what keeps them the same size.
export const CHART_WIDTH = 600;
export const CHART_HEIGHT = 180;
// `left` is the gutter's floor, wide enough for the common axis labels
// ("512 MiB", "0.110%", "169ms") at the axis font, so charts stacked in one
// card start their plots on the same x and their time axes line up; only a
// longer label widens one chart's gutter.
export const CHART_PAD = { top: 8, right: 8, bottom: 20, left: 56 };
/** A chart never gets shorter than this, however narrow its card. */
export const CHART_MIN_HEIGHT = 140;
/** Nor taller than this, however wide: stacked charts stay comparable on
 *  one screen instead of each filling it. */
export const CHART_MAX_HEIGHT = 200;

/**
 * The populated chart is a width-100% SVG with a CHART_WIDTH×CHART_HEIGHT
 * viewBox, so it scales to that ratio (256px tall at 854px, 95px at 316px).
 * Every non-data frame — loading, empty, unavailable, error — takes the same
 * ratio, so a section never changes height when its data arrives (w4/m209).
 */
export const CHART_FRAME_STYLE = {
  aspectRatio: `${CHART_WIDTH} / ${CHART_HEIGHT}`,
  minHeight: CHART_MIN_HEIGHT,
  maxHeight: CHART_MAX_HEIGHT,
} as const;

/** The size a chart's SVG box draws in. */
export interface ChartBox {
  width: number;
  height: number;
  /** False until the browser measured the box (server render, first paint). */
  measured: boolean;
}

const UNMEASURED_BOX: ChartBox = {
  width: CHART_WIDTH,
  height: CHART_HEIGHT,
  measured: false,
};

/**
 * The chart SVG's own CSS box, in pixels. The box itself is sized by CSS
 * (CHART_FRAME_STYLE, the same as every placeholder frame, so nothing moves
 * when it is measured); the chart draws its geometry in these pixels, so tick
 * labels stay at the axis font on a phone instead of shrinking with a scaled
 * 600-wide canvas to an unreadable few pixels. Unmeasured (server render, a
 * test without ResizeObserver) it is the nominal 600×180 canvas.
 */
export function useChartBox<T extends Element>() {
  // A callback ref: the SVG mounts only once there is data, which can be
  // long after the chart component itself mounted on an empty frame.
  const [el, ref] = useState<T | null>(null);
  const [box, setBox] = useState<ChartBox>(UNMEASURED_BOX);
  useLayoutEffect(() => {
    if (!el || typeof ResizeObserver === "undefined") return;
    const apply = (width: number, height: number) => {
      const w = Math.round(width);
      const h = Math.round(height);
      if (w <= 0 || h <= 0) return;
      setBox((cur) =>
        cur.measured && cur.width === w && cur.height === h
          ? cur
          : { width: w, height: h, measured: true },
      );
    };
    const rect = el.getBoundingClientRect();
    apply(rect.width, rect.height);
    const observer = new ResizeObserver((entries) => {
      const r = entries[0]?.contentRect;
      if (r) apply(r.width, r.height);
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, [el]);
  return { ref, box };
}

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
// Round steps a time axis may tick on, finest first.
const TIME_STEPS = [
  5 * MINUTE,
  10 * MINUTE,
  15 * MINUTE,
  30 * MINUTE,
  HOUR,
  2 * HOUR,
  3 * HOUR,
  6 * HOUR,
  12 * HOUR,
  DAY,
  2 * DAY,
  7 * DAY,
];
// Clear space between two neighbouring time labels.
const TIME_LABEL_GAP = 16;

/** The multiples of `step` in local wall-clock time within [minT, maxT]. */
function alignedTimes(minT: number, maxT: number, step: number): number[] {
  // Local wall clock = instant - timezone offset; whole-hour and whole-day
  // steps then land on the hour and on midnight where the viewer is.
  const offset = new Date(minT).getTimezoneOffset() * MINUTE;
  const times: number[] = [];
  for (
    let t = Math.ceil((minT - offset) / step) * step + offset;
    t <= maxT;
    t += step
  ) {
    times.push(t);
  }
  return times;
}

/** One labelled tick on a chart's time axis. */
export interface TimeTick {
  x: number;
  label: string;
}

/**
 * The time axis: ticks on round boundaries (on the hour, every 6 hours, at
 * midnight…), the finest step whose labels — measured at the axis font —
 * keep a clear gap between neighbours, so labels never run together on a
 * phone or in a longer locale. A label that would cross the canvas edge is
 * dropped rather than clamped against its neighbour.
 */
export function axisTimeTicks(
  minT: number,
  maxT: number,
  xFor: (t: number) => number,
  width: number,
  format: (t: number, stepMs: number) => string,
): TimeTick[] {
  if (!(maxT > minT)) return [];
  const pxPerMs = (xFor(maxT) - xFor(minT)) / (maxT - minT);
  for (const [i, step] of TIME_STEPS.entries()) {
    const last = i === TIME_STEPS.length - 1;
    const spacing = step * pxPerMs;
    const times = alignedTimes(minT, maxT, step);
    if (times.length === 0) continue;
    // One label's width bounds the widest from below, so a step too fine
    // for it is rejected before the whole axis is formatted and measured
    // (a 30-day range is ~8,600 five-minute ticks).
    if (
      !last &&
      spacing < tickLabelWidth(format(times[0], step)) + TIME_LABEL_GAP
    ) {
      continue;
    }
    const labels = times.map((t) => format(t, step));
    const widths = labels.map(tickLabelWidth);
    if (!last && spacing < Math.max(...widths) + TIME_LABEL_GAP) continue;
    return times.flatMap((t, j) => {
      const x = xFor(t);
      const half = widths[j] / 2;
      return x - half >= 0 && x + half <= width
        ? [{ x, label: labels[j] }]
        : [];
    });
  }
  return [];
}

/**
 * A service chart section keeps a fixed 24px event lane above its plot, empty
 * or not, so the section is one height whether markers, data or neither have
 * arrived (w4/446). MetricSection provides it; every frame that can stand in
 * for the chart reserves it.
 */
export const EventLaneContext = createContext(false);

/** Whether this chart frame sits in a section with an event lane. */
export function useEventLane() {
  return useContext(EventLaneContext);
}

// Tick labels are end-anchored TICK_LABEL_GAP left of the plot, at the
// 11px axis font (real pixels: the chart draws in its measured box), and
// keep TICK_INSET clear of the canvas edge.
const TICK_FONT_PX = 11;
const TICK_LABEL_GAP = 6;
const TICK_INSET = 4;
let measureCtx: CanvasRenderingContext2D | null | undefined;

/** A tick label's width at the axis font, in viewBox units. */
export function tickLabelWidth(label: string): number {
  if (measureCtx === undefined && typeof document !== "undefined") {
    try {
      measureCtx = document.createElement("canvas").getContext("2d");
      if (measureCtx) {
        const family =
          getComputedStyle(document.body).fontFamily || "sans-serif";
        measureCtx.font = `${TICK_FONT_PX}px ${family}`;
      }
    } catch {
      measureCtx = null;
    }
  }
  const measured = measureCtx?.measureText(label).width;
  // Without a canvas, a bounded estimate: 0.6em per character.
  return measured && measured > 0
    ? measured
    : label.length * TICK_FONT_PX * 0.6;
}

/**
 * The plot's left gutter for these formatted y-tick labels: wide enough for
 * the longest at the axis font plus its gap and inset, never narrower than
 * CHART_PAD.left (w4/m209 — "536.7 KiB" no longer starts before the canvas).
 */
export function axisLeftPad(labels: string[]): number {
  const widest = Math.max(0, ...labels.map(tickLabelWidth));
  return Math.max(
    CHART_PAD.left,
    Math.ceil(widest + TICK_LABEL_GAP + TICK_INSET),
  );
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
