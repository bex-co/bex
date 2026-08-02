export type UsageCoverageState =
  "complete" | "partial" | "unknown" | "unavailable" | "healthy-empty";

export type UsageCoverageWire = {
  state?: string | null;
  through?: string | null;
  degradedSources?: readonly (string | null)[] | null;
};

export type UsageRowWire = {
  kind?: string | null;
  total?: number | null;
};

export type ServiceUsageWire = {
  rows?: readonly (UsageRowWire | null)[] | null;
};

export type UsageSummaryWire = {
  period?: string | null;
  coverage?: UsageCoverageWire | null;
  services?: readonly (ServiceUsageWire | null)[] | null;
};

export type UsageTotal = {
  /** Backend meter id. Unknown future meters remain visible under this id. */
  kind: string;
  total: number;
};

export type UsageGlance = {
  period: string | null;
  state: UsageCoverageState;
  /** A refresh failed after this payload was cached; evidence stays visible. */
  refreshUnavailable: boolean;
  through: string | null;
  degradedSources: string[];
  totals: UsageTotal[];
};

const MAX_DEGRADED_SOURCES = 8;
const MAX_DEGRADED_SOURCE_LENGTH = 48;
const SAFE_DEGRADED_SOURCE = /^[a-z0-9][a-z0-9._:-]*$/i;
const METER_ORDER: readonly string[] = [
  "instance_seconds",
  "egress_bytes",
  "build_seconds",
  "storage_gb_seconds",
  "sandbox_compute_seconds",
];

/**
 * Converts the API payload into honest mobile display state. In particular,
 * absent/null totals are discarded instead of becoming zero, and coverage
 * that predates the explicit contract remains unknown even when old positive
 * totals are available.
 */
export function buildUsageGlance(input: {
  summary: UsageSummaryWire | null | undefined;
  unavailable?: boolean;
}): UsageGlance {
  const totals = aggregateTotals(input.summary?.services);
  const coverage = input.summary?.coverage;
  const reported = normalizeCoverageState(coverage?.state);
  const state: UsageCoverageState = input.summary
    ? reported === "complete" && hasHealthyEmptyEvidence(input.summary.services)
      ? "healthy-empty"
      : reported
    : input.unavailable
      ? "unavailable"
      : "unknown";

  return {
    period: normalizePeriod(input.summary?.period),
    state,
    refreshUnavailable: Boolean(input.summary && input.unavailable),
    through: normalizeTimestamp(coverage?.through),
    degradedSources: normalizeDegradedSources(coverage?.degradedSources),
    totals,
  };
}

function hasHealthyEmptyEvidence(
  services: readonly (ServiceUsageWire | null)[] | null | undefined,
): boolean {
  if (!services) return false;
  for (const service of services) {
    if (!service?.rows) return false;
    for (const row of service.rows) {
      const kind = row?.kind?.trim();
      const total = row?.total;
      if (
        !kind ||
        typeof total !== "number" ||
        !Number.isFinite(total) ||
        total !== 0
      ) {
        return false;
      }
    }
  }
  return true;
}

export function aggregateTotals(
  services: readonly (ServiceUsageWire | null)[] | null | undefined,
): UsageTotal[] {
  const byKind = new Map<string, number>();
  for (const service of services ?? []) {
    for (const row of service?.rows ?? []) {
      const kind = row?.kind?.trim();
      const total = row?.total;
      if (
        !kind ||
        typeof total !== "number" ||
        !Number.isFinite(total) ||
        total < 0
      ) {
        continue;
      }
      byKind.set(kind, (byKind.get(kind) ?? 0) + total);
    }
  }
  return [...byKind.entries()]
    .sort(
      ([left], [right]) =>
        meterOrder(left) - meterOrder(right) || left.localeCompare(right),
    )
    .map(([kind, total]) => ({ kind, total }));
}

export function formatUsageTotal(kind: string, total: number): string {
  switch (kind) {
    case "egress_bytes":
      return formatBytes(total);
    case "instance_seconds":
    case "build_seconds":
      return formatHours(total / 3_600);
    case "storage_gb_seconds":
      return formatHours(total / 3_600, "GB-h");
    case "sandbox_compute_seconds":
      return formatHours(total / 3_600_000, "vCPU-h");
    default:
      return total.toLocaleString(undefined, { maximumFractionDigits: 2 });
  }
}

function normalizeCoverageState(
  value: string | null | undefined,
): "complete" | "partial" | "unknown" {
  switch (value?.trim().toLowerCase()) {
    case "complete":
      return "complete";
    case "partial":
      return "partial";
    default:
      return "unknown";
  }
}

function normalizeTimestamp(value: string | null | undefined): string | null {
  if (!value || !Number.isFinite(Date.parse(value))) return null;
  return new Date(value).toISOString();
}

function normalizePeriod(value: string | null | undefined): string | null {
  return value && /^\d{4}-(0[1-9]|1[0-2])$/.test(value) ? value : null;
}

function normalizeDegradedSources(
  values: readonly (string | null)[] | null | undefined,
): string[] {
  const normalized = new Set<string>();
  for (const value of values ?? []) {
    const source = value?.trim();
    if (
      !source ||
      source.length > MAX_DEGRADED_SOURCE_LENGTH ||
      !SAFE_DEGRADED_SOURCE.test(source)
    ) {
      continue;
    }
    normalized.add(source);
  }
  return [...normalized]
    .sort((left, right) => left.localeCompare(right))
    .slice(0, MAX_DEGRADED_SOURCES);
}

function meterOrder(kind: string): number {
  const index = METER_ORDER.indexOf(kind);
  return index < 0 ? METER_ORDER.length : index;
}

function formatBytes(value: number): string {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let shown = value;
  let index = 0;
  while (shown >= 1_024 && index < units.length - 1) {
    shown /= 1_024;
    index += 1;
  }
  return `${shown.toLocaleString(undefined, {
    maximumFractionDigits: index === 0 ? 0 : 1,
  })} ${units[index]}`;
}

function formatHours(value: number, unit = "h"): string {
  return `${value.toLocaleString(undefined, {
    maximumFractionDigits: value < 10 ? 2 : 1,
  })} ${unit}`;
}
