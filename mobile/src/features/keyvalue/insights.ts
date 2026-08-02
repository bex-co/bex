import {
  adaptMetricSeries,
  type MetricSnapshot,
  type MetricWireSeries,
} from "../metrics/series";
import { isLifecycleSuspended } from "../services/lifecycle";

export type KeyValueConnectionHealth =
  "available" | "creating" | "unavailable" | "suspended" | "unknown";

export type KeyValueInsightMetric =
  "disk" | "diskCapacity" | "memory" | "connections";

export type KeyValueMetricFailure = "unavailable" | "error" | null;

export type KeyValueInsightsWire = Partial<
  Record<KeyValueInsightMetric, readonly (MetricWireSeries | null)[] | null>
>;

export type KeyValueInsightSnapshot = {
  disk: MetricSnapshot;
  diskCapacity: MetricSnapshot;
  memory: MetricSnapshot;
  connections: MetricSnapshot;
  /** Newest real scrape sample across the card; null is never treated as now. */
  latestAt: string | null;
  /** Null when either side is absent/invalid, rather than a fabricated zero. */
  diskUsedPercent: number | null;
};

export function keyValueConnectionHealth(resource: {
  status: string | null | undefined;
  suspended: boolean | string | null | undefined;
}): KeyValueConnectionHealth {
  if (isLifecycleSuspended(resource.suspended)) return "suspended";
  switch (resource.status?.trim().toLowerCase()) {
    case "available":
      return "available";
    case "creating":
      return "creating";
    case "unavailable":
      return "unavailable";
    default:
      return "unknown";
  }
}

export function buildKeyValueInsightSnapshot(
  raw: KeyValueInsightsWire | null | undefined,
): KeyValueInsightSnapshot {
  const disk = adaptMetricSeries(raw?.disk);
  const diskCapacity = adaptMetricSeries(raw?.diskCapacity);
  const memory = adaptMetricSeries(raw?.memory);
  const connections = adaptMetricSeries(raw?.connections);
  const timestamps = [disk, diskCapacity, memory, connections]
    .flatMap((metric) => metric.points.map((point) => point.timestamp))
    .filter((timestamp) => Number.isFinite(Date.parse(timestamp)))
    .sort((left, right) => Date.parse(left) - Date.parse(right));
  const used = disk.current;
  const capacity = diskCapacity.current;
  const diskUsedPercent =
    used != null &&
    capacity != null &&
    Number.isFinite(used) &&
    Number.isFinite(capacity) &&
    used >= 0 &&
    capacity > 0
      ? (used / capacity) * 100
      : null;
  return {
    disk,
    diskCapacity,
    memory,
    connections,
    latestAt: timestamps.at(-1) ?? null,
    diskUsedPercent,
  };
}

/**
 * Classifies an Apollo partial-response failure for one aliased metric.
 * Path-specific sibling failures leave this alias alone. A failure for this
 * alias or the whole transport still degrades cached samples rather than
 * presenting them as current.
 */
export function keyValueMetricFailure(
  error: unknown,
  alias: KeyValueInsightMetric,
  _hasPayload: boolean,
): KeyValueMetricFailure {
  if (error == null) return null;
  const errors = nestedGraphQLErrors(error);
  const relevant = errors.filter((item) => {
    const path = errorPath(item);
    return path.length === 0 || path[0] === alias;
  });
  if (errors.length > 0 && relevant.length === 0) return null;
  const messages = (relevant.length ? relevant : [error])
    .map(errorMessage)
    .filter(Boolean);
  return messages.some((message) =>
    message.toLowerCase().includes("metrics source not configured"),
  )
    ? "unavailable"
    : "error";
}

function nestedGraphQLErrors(error: unknown): unknown[] {
  if (!error || typeof error !== "object") return [];
  for (const key of ["errors", "graphQLErrors"] as const) {
    const value = (error as Record<string, unknown>)[key];
    if (Array.isArray(value)) return value;
  }
  return [];
}

function errorPath(error: unknown): readonly unknown[] {
  if (!error || typeof error !== "object") return [];
  const path = (error as Record<string, unknown>).path;
  return Array.isArray(path) ? path : [];
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (error && typeof error === "object") {
    const message = (error as Record<string, unknown>).message;
    return typeof message === "string" ? message : "";
  }
  return typeof error === "string" ? error : "";
}
