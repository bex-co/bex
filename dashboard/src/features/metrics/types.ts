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
  // bex extensions (w3/m10): the App's configured autoscale-target utilization
  // (w1/m20), alongside current cpu/memory usage. No Render equivalent.
  "cpu_target",
  "memory_target",
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
  cpu_target: "CPU_TARGET",
  memory_target: "MEMORY_TARGET",
};

// Datastore metric ids (w3/m10) — the Database/KeyValue-scoped sibling of
// MetricId. All bex extensions (no Render equivalent): mirrors
// lego/backend/internal/metrics/datastore.go's Metric* constants.
export const DATASTORE_METRIC_IDS = [
  "disk",
  "disk_capacity",
  "db_connections",
  "replication_lag",
  "kv_memory",
  "kv_connections",
] as const;

export type DatastoreMetricId = (typeof DATASTORE_METRIC_IDS)[number];

export type DatastoreKind = "database" | "keyvalue";

// Mirrors lego/backend/internal/metrics/graphql.go's datastoreMetricNames.
export const RENDER_DATASTORE_METRIC_NAMES: Record<DatastoreMetricId, string> =
  {
    disk: "DISK",
    disk_capacity: "DISK_CAPACITY",
    db_connections: "DB_CONNECTIONS",
    replication_lag: "REPLICATION_LAG",
    kv_memory: "MEMORY",
    kv_connections: "CONNECTIONS",
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
