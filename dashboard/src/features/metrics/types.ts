// bex-api's metric ids (operator/internal/api/metrics.go) — REST's
// `/v1/metrics/{segment}` path segment. GraphQL instead sends Render's
// uppercase `name` enum (RENDER_METRIC_NAMES below) inside a MetricsQueryInput.
export const METRIC_IDS = [
  "cpu",
  "memory",
  "cpu_limit",
  "memory_limit",
  "instance_count",
  "http_requests",
  "http_latency",
  "bandwidth",
] as const;

export type MetricId = (typeof METRIC_IDS)[number];

// Mirrors operator/internal/api/graphql.go's renderMetricNames — the GraphQL
// `metrics(query: { name })` value for each bex metric id, captured live from
// Render's dashboard traffic.
export const RENDER_METRIC_NAMES: Record<MetricId, string> = {
  cpu: "CPU",
  memory: "MEMORY",
  cpu_limit: "CPU_LIMIT",
  memory_limit: "MEMORY_LIMIT",
  instance_count: "INSTANCES",
  http_requests: "HTTP_REQUESTS",
  http_latency: "HTTP_LATENCY",
  bandwidth: "BANDWIDTH",
};

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
