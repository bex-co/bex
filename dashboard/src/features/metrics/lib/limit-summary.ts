import type { ChartSeries } from "@/features/metrics/types";

/**
 * One App limit as the header can honestly state it: no limit series at all,
 * one uniform value across the selected replicas, or mixed per-replica
 * limits. Computed from the per-instance _limit series' latest points.
 */
export type LimitSummary =
  | { kind: "none" }
  | { kind: "single"; value: number }
  | { kind: "vary" };

export function summarizeLimits(series: ChartSeries[]): LimitSummary {
  const values = series
    .map((s) =>
      s.points.length > 0 ? s.points[s.points.length - 1].value : null,
    )
    .filter((v): v is number => v != null);
  if (values.length === 0) return { kind: "none" };
  return values.every((v) => v === values[0])
    ? { kind: "single", value: values[0] }
    : { kind: "vary" };
}
