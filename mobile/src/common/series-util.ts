export type SeriesPoint = { timestamp: string; value: number };
export type TimeRange = "1H" | "6H" | "24H" | "7D" | "30D";

export const TIME_RANGES: TimeRange[] = ["1H", "6H", "24H", "7D", "30D"];
const RANGE_MS: Record<TimeRange, number> = {
  "1H": 60 * 60 * 1000,
  "6H": 6 * 60 * 60 * 1000,
  "24H": 24 * 60 * 60 * 1000,
  "7D": 7 * 24 * 60 * 60 * 1000,
  "30D": 30 * 24 * 60 * 60 * 1000,
};

export function normalizeSeries(points: readonly SeriesPoint[]): SeriesPoint[] {
  const byTimestamp = new Map<string, number>();
  for (const point of points) {
    if (
      !Number.isFinite(point.value) ||
      Number.isNaN(Date.parse(point.timestamp))
    )
      continue;
    byTimestamp.set(point.timestamp, point.value);
  }
  return [...byTimestamp]
    .map(([timestamp, value]) => ({ timestamp, value }))
    .sort((a, b) => a.timestamp.localeCompare(b.timestamp));
}

export function filterSeriesByRange(
  points: readonly SeriesPoint[],
  range: TimeRange,
): SeriesPoint[] {
  const normalized = normalizeSeries(points);
  const latest = normalized.at(-1);
  if (!latest) return [];
  const cutoff = Date.parse(latest.timestamp) - RANGE_MS[range];
  return normalized.filter((point) => Date.parse(point.timestamp) >= cutoff);
}

export function alignSeries(series: readonly (readonly SeriesPoint[])[]): {
  timestamps: string[];
  values: Array<Array<number | null>>;
} {
  const normalized = series.map(normalizeSeries);
  const timestamps = [
    ...new Set(
      normalized.flatMap((points) => points.map((point) => point.timestamp)),
    ),
  ].sort();
  return {
    timestamps,
    values: normalized.map((points) => {
      const byTimestamp = new Map(
        points.map((point) => [point.timestamp, point.value]),
      );
      return timestamps.map((timestamp) => byTimestamp.get(timestamp) ?? null);
    }),
  };
}
