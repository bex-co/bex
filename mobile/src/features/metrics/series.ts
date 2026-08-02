export type MetricWireSeries = {
  unit: string | null;
  labels:
    readonly ({ field: string | null; value: string | null } | null)[] | null;
  values:
    readonly ({ time: string | null; value: number | null } | null)[] | null;
};

export type MetricPoint = { timestamp: string; value: number };

export type MetricSnapshot = {
  unit: string;
  points: MetricPoint[];
  current: number | null;
  degradedSources: string[];
  partial: boolean;
};

export function adaptMetricSeries(
  raw: readonly (MetricWireSeries | null)[] | null | undefined,
): MetricSnapshot {
  const series = (raw ?? []).filter(
    (candidate): candidate is MetricWireSeries => Boolean(candidate),
  );
  const totals = new Map<number, number>();
  let discarded = 0;
  let accepted = 0;
  const degraded = new Set<string>();

  for (const item of series) {
    const labels = Object.fromEntries(
      (item.labels ?? []).flatMap((label) =>
        label?.field && label.value ? [[label.field, label.value]] : [],
      ),
    );
    for (const source of (labels.degraded_sources ?? "").split(",")) {
      if (source.trim()) degraded.add(source.trim());
    }
    for (const point of item.values ?? []) {
      const time = point?.time ? Date.parse(point.time) : Number.NaN;
      if (!Number.isFinite(time) || !Number.isFinite(point?.value)) {
        discarded += 1;
        continue;
      }
      accepted += 1;
      totals.set(time, (totals.get(time) ?? 0) + (point?.value ?? 0));
    }
  }

  const points = [...totals.entries()]
    .sort(([left], [right]) => left - right)
    .map(([time, value]) => ({
      timestamp: new Date(time).toISOString(),
      value,
    }));
  const units = new Set(series.map((item) => item.unit).filter(Boolean));
  return {
    unit: series.find((item) => item.unit)?.unit ?? "",
    points,
    current: points.at(-1)?.value ?? null,
    degradedSources: [...degraded],
    partial:
      discarded > 0 ||
      (accepted > 0 &&
        series.some((item) => (item.values?.length ?? 0) === 0)) ||
      units.size > 1,
  };
}

export function newestMetricTimestamp(
  snapshots: readonly Pick<MetricSnapshot, "points">[],
): string | null {
  let newest: { timestamp: string; time: number } | null = null;
  for (const point of snapshots.flatMap((snapshot) => snapshot.points)) {
    const time = Date.parse(point.timestamp);
    if (Number.isFinite(time) && (!newest || time > newest.time)) {
      newest = { timestamp: point.timestamp, time };
    }
  }
  return newest?.timestamp ?? null;
}

export function formatMetricValue(unit: string, value: number): string {
  switch (unit) {
    case "cpu":
      return `${value.toFixed(3)} cores`;
    case "bytes":
      return formatBytes(value);
    case "percentage":
      return `${value.toFixed(1)}%`;
    case "seconds":
      return `${Math.round(value * 1_000)} ms`;
    case "count":
      return value.toLocaleString(undefined, { maximumFractionDigits: 1 });
    default:
      return value.toLocaleString(undefined, { maximumFractionDigits: 2 });
  }
}

function formatBytes(value: number): string {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let shown = value;
  let index = 0;
  while (Math.abs(shown) >= 1024 && index < units.length - 1) {
    shown /= 1024;
    index += 1;
  }
  return `${shown.toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
}
