import type { MetricId } from "@/features/metrics/types";

/** Formats a metric's value for its unit, matching bex-api's `unit` field. */
export function formatMetricValue(unit: string, value: number): string {
  switch (unit) {
    case "cpu":
      return `${value.toFixed(3)} cores`;
    case "bytes":
      return formatBytes(value);
    case "seconds":
      return `${(value * 1000).toFixed(0)} ms`;
    case "percentage":
      return `${value.toFixed(1)}%`;
    case "count":
      return formatCount(value);
    default:
      return String(value);
  }
}

/** Compact axis/tooltip value — shorter than formatMetricValue's full label. */
export function formatMetricShort(unit: string, value: number): string {
  switch (unit) {
    case "cpu":
      return value.toFixed(2);
    case "bytes":
      return formatBytes(value, 1);
    case "seconds":
      return `${(value * 1000).toFixed(0)}ms`;
    case "percentage":
      return `${value.toFixed(0)}%`;
    case "count":
      return formatCount(value);
    default:
      return String(value);
  }
}

/** Formats a monthToDateBandwidth MB figure using the same byte-unit scaling. */
export function formatMegabytes(mb: number): string {
  return formatBytes(mb * 1024 * 1024);
}

function formatBytes(value: number, digits = 0): string {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let v = value;
  let i = 0;
  while (Math.abs(v) >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(i === 0 ? 0 : digits)} ${units[i]}`;
}

function formatCount(value: number): string {
  if (Math.abs(value) >= 1000) {
    return `${(value / 1000).toFixed(1)}K`;
  }
  // Integer request counts (http_requests after w4/m108) and whole instance
  // counts render without a decimal; rare fractional leftovers from
  // increase() extrapolation keep one digit.
  return value.toFixed(value % 1 === 0 ? 0 : 1);
}

/** Human label for a metric id, matching Render's naming for each chart. */
export const METRIC_LABELS: Record<MetricId, string> = {
  cpu: "CPU",
  memory: "Memory",
  cpu_limit: "CPU Limit",
  memory_limit: "Memory Limit",
  instance_count: "Total Instances",
  http_requests: "Total Requests",
  http_latency: "Response Times",
  bandwidth: "Outbound Bandwidth",
  cpu_target: "CPU Target",
  memory_target: "Memory Target",
};
