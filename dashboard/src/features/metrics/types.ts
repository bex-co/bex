// bex-api's metric ids (operator/internal/api/metrics.go) — the `metric` arg
// to the GraphQL `metrics(...)` query and REST's `/v1/metrics/{segment}` path.
export const METRIC_IDS = [
  "cpu",
  "memory",
  "instance_count",
  "http_requests",
  "http_latency",
  "bandwidth",
] as const;

export type MetricId = (typeof METRIC_IDS)[number];

// A metric point with guaranteed (non-null) fields — what charts consume.
// bex-api's GraphQL schema marks every field nullable (Apollo codegen default),
// but a well-formed series never actually omits timestamp/value; this is the
// boundary where we assert that and hand components a clean shape.
export interface ChartPoint {
  timestamp: string;
  value: number;
}

export interface ChartSeries {
  unit: string;
  /** Render's {field, value} labels, e.g. instance/resource. */
  labels: Record<string, string>;
  points: ChartPoint[];
}
