export const POSTGRES_INSIGHT_STALE_AFTER_MS = 65_000;

export type PostgresInsightState =
  "loading" | "empty" | "current" | "stale" | "degraded" | "unavailable";

export function postgresInsightState(input: {
  hasData: boolean;
  hasError: boolean;
  observedAt: number | null;
  now?: number;
  staleAfterMs?: number;
}): PostgresInsightState {
  if (input.hasError) return input.hasData ? "degraded" : "unavailable";
  if (!input.hasData || input.observedAt == null) return "loading";
  const now = input.now ?? Date.now();
  const staleAfterMs = input.staleAfterMs ?? POSTGRES_INSIGHT_STALE_AFTER_MS;
  return now - input.observedAt >= staleAfterMs ? "stale" : "current";
}

export type PostgresProcessRow = {
  state?: string | null;
  waitEventType?: string | null;
  durationSeconds?: number | null;
};

export type PostgresProcessSummary = {
  total: number;
  active: number;
  waiting: number;
  longestSeconds: number | null;
};

export function summarizePostgresProcesses(
  rows: readonly (PostgresProcessRow | null)[] | null | undefined,
): PostgresProcessSummary {
  const valid = (rows ?? []).filter(
    (row): row is PostgresProcessRow => row != null,
  );
  const durations = valid.flatMap((row) =>
    typeof row.durationSeconds === "number" &&
    Number.isFinite(row.durationSeconds) &&
    row.durationSeconds >= 0
      ? [row.durationSeconds]
      : [],
  );
  return {
    total: valid.length,
    active: valid.filter((row) => row.state?.toLowerCase() === "active").length,
    waiting: valid.filter((row) => Boolean(row.waitEventType?.trim())).length,
    longestSeconds: durations.length ? Math.max(...durations) : null,
  };
}

export type PostgresTableSizeRow = {
  schema?: string | null;
  name?: string | null;
  sizePretty?: string | null;
};

export type PostgresTableScanRow = {
  schema?: string | null;
  name?: string | null;
  seqScans?: number | null;
  indexScans?: number | null;
  deadRows?: number | null;
};

export type PostgresTableInsight = {
  key: string;
  label: string;
  sizePretty: string | null;
  seqScans: number | null;
  indexScans: number | null;
  deadRows: number | null;
};

/**
 * Joins the existing read-only table-size and scan snapshots without adding a
 * new backend surface. Scan-heavy tables sort first; size-only tables fill the
 * remaining compact slots. Missing values stay absent rather than becoming a
 * misleading zero, except for counters the backend explicitly returned.
 */
export function compactPostgresTableInsights(
  sizes: readonly (PostgresTableSizeRow | null)[] | null | undefined,
  scans: readonly (PostgresTableScanRow | null)[] | null | undefined,
  limit = 3,
): PostgresTableInsight[] {
  const byKey = new Map<string, PostgresTableInsight>();
  for (const row of sizes ?? []) {
    const identity = tableIdentity(row);
    if (!identity) continue;
    byKey.set(identity.key, {
      key: identity.key,
      label: identity.label,
      sizePretty: row?.sizePretty?.trim() || null,
      seqScans: null,
      indexScans: null,
      deadRows: null,
    });
  }
  for (const row of scans ?? []) {
    const identity = tableIdentity(row);
    if (!identity) continue;
    const current = byKey.get(identity.key);
    byKey.set(identity.key, {
      key: identity.key,
      label: identity.label,
      sizePretty: current?.sizePretty ?? null,
      seqScans: finiteCounter(row?.seqScans),
      indexScans: finiteCounter(row?.indexScans),
      deadRows: finiteCounter(row?.deadRows),
    });
  }
  return [...byKey.values()]
    .sort(
      (left, right) =>
        (right.seqScans ?? -1) - (left.seqScans ?? -1) ||
        (right.deadRows ?? -1) - (left.deadRows ?? -1) ||
        left.label.localeCompare(right.label),
    )
    .slice(0, Math.max(0, limit));
}

function tableIdentity(
  row: {
    schema?: string | null;
    name?: string | null;
  } | null,
): { key: string; label: string } | null {
  const name = row?.name?.trim();
  if (!name) return null;
  const schema = row?.schema?.trim();
  const label = schema ? `${schema}.${name}` : name;
  return { key: label, label };
}

function finiteCounter(value: number | null | undefined): number | null {
  return typeof value === "number" && Number.isFinite(value) && value >= 0
    ? value
    : null;
}
