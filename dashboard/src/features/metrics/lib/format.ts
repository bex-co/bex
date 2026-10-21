import type { MetricId } from "@/features/metrics/types";

/** Formats a metric's value for its unit, matching bex-api's `unit` field. */
export function formatMetricValue(unit: string, value: number): string {
  switch (unit) {
    case "cpu":
      return `${toFixedNonzero(value, 3)} cores`;
    case "bytes":
      return formatBytes(value);
    case "seconds":
      return `${(value * 1000).toFixed(0)} ms`;
    case "percentage":
      return `${toFixedNonzero(value, 1)}%`;
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
      return toFixedNonzero(value, 2);
    case "bytes":
      return formatBytes(value, 1);
    case "seconds":
      return `${(value * 1000).toFixed(0)}ms`;
    case "percentage":
      return `${toFixedNonzero(value, 0)}%`;
    case "count":
      return formatCount(value);
    default:
      return String(value);
  }
}

/**
 * Labels for one axis' tick values (w4/381). The compact formatter rounds
 * each tick independently, so a narrow range (midpoint ≈0.7%, max ≈1.4%)
 * labels two gridlines `1%`. Here the whole axis shares the smallest extra
 * precision that makes every differing value's label distinct; ordinary
 * magnitudes keep the compact labels unchanged, and a flat axis (all
 * values equal) keeps its single repeated label rather than inventing
 * precision. Capped escalation: past a few extra digits the values are
 * indistinguishable at chart scale anyway.
 */
export function formatAxisTickLabels(unit: string, values: number[]): string[] {
  const base = values.map((v) => formatMetricShort(unit, v));
  if (new Set(base).size === base.length || new Set(values).size <= 1) {
    return base;
  }
  for (let extra = 1; extra <= 4; extra++) {
    const labels = values.map((v) => formatShortExtra(unit, v, extra));
    if (new Set(labels).size === labels.length) {
      return labels;
    }
  }
  return values.map((v) => formatShortExtra(unit, v, 4));
}

/** formatMetricShort with `extra` more fractional digits than the base. */
function formatShortExtra(unit: string, value: number, extra: number): string {
  switch (unit) {
    case "cpu":
      return toFixedNonzero(value, 2 + extra);
    case "bytes":
      return formatBytes(value, 1 + extra);
    case "seconds":
      return `${(value * 1000).toFixed(extra)}ms`;
    case "percentage":
      return `${toFixedNonzero(value, extra)}%`;
    case "count":
      return Math.abs(value) >= 1000
        ? `${(value / 1000).toFixed(1 + extra)}K`
        : value.toFixed(extra);
    default:
      return value.toFixed(extra);
  }
}

/**
 * `value.toFixed(digits)`, except a nonzero value that would round to zero
 * keeps three significant digits instead (`0.000125`, `0.0710`; exponent
 * notation like `1.00e-9` below 1e-6 keeps the label bounded). A lightly used
 * service's CPU/percentage readings stay legible and its axis ticks distinct,
 * while ordinary magnitudes and true zero keep the fixed convention.
 */
function toFixedNonzero(value: number, digits: number): string {
  const fixed = value.toFixed(digits);
  if (value === 0 || !Number.isFinite(value) || Number(fixed) !== 0) {
    return fixed;
  }
  return value.toPrecision(3);
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
  // Whole bytes stay whole at the base precisions (digits ≤ 1); an axis
  // asking for more (formatAxisTickLabels) gets decimals even in B (w4/381).
  return `${v.toFixed(i === 0 && digits <= 1 ? 0 : digits)} ${units[i]}`;
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
