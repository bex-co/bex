import type { TimeTick } from "@/features/metrics/components/chart-layout";

/** The class every axis label shares: the axis font (TICK_FONT_PX). */
export const AXIS_LABEL_CLASS =
  "fill-muted-foreground text-[11px] tabular-nums";

/** A chart's x-axis labels, centred under their ticks on the bottom edge. */
export function ChartTimeAxis({
  ticks,
  height,
}: {
  ticks: TimeTick[];
  /** The chart's drawn height. */
  height: number;
}) {
  return (
    <>
      {ticks.map((tick, i) => (
        <text
          key={i}
          x={tick.x}
          y={height - 4}
          textAnchor="middle"
          className={AXIS_LABEL_CLASS}
        >
          {tick.label}
        </text>
      ))}
    </>
  );
}
