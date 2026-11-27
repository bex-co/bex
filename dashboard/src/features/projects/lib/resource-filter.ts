import type { ResourceRow } from "@/features/projects/types";
import { canonicalEnvironmentLinkId } from "@/features/environments/lib/link-id";

/**
 * The resource lists a project page joins membership against (w4/m227). Each
 * answers on its own, so each can be unread on its own.
 */
export type ProjectResourceFamily =
  "services" | "databases" | "keyvalues" | "envgroups";

/**
 * An environment section's switcher, as Render's project page draws it
 * (w1/m682): All, Services (every service, Postgres, Key Value and sandbox
 * group) and Env Groups.
 */
export const ENVIRONMENT_RESOURCE_FILTERS = [
  "all",
  "services",
  "envgroups",
] as const;
export type EnvironmentResourceFilter =
  (typeof ENVIRONMENT_RESOURCE_FILTERS)[number];

export function parseEnvironmentResourceFilter(
  value: unknown,
): EnvironmentResourceFilter {
  return typeof value === "string" &&
    ENVIRONMENT_RESOURCE_FILTERS.includes(value as EnvironmentResourceFilter)
    ? (value as EnvironmentResourceFilter)
    : "all";
}

/**
 * The project page's search before w1/m682 (`?env=`, `?q=`, `?kind=`). Each
 * key is undefined, not omitted, when rejected: the router merges this over
 * the raw search, so an omitted key would reach readers raw (w5/078). The
 * page no longer reads them; a link carrying any of them redirects once.
 */
export interface LegacyProjectSearch {
  env?: string;
  q?: string;
  kind?: string;
}

export function parseLegacyProjectSearch(
  search: Record<string, unknown>,
): LegacyProjectSearch {
  const text = (value: unknown) =>
    typeof value === "string" && value ? value : undefined;
  const env = text(search.env);
  return {
    env: env ? canonicalEnvironmentLinkId(env) : undefined,
    q: text(search.q),
    kind: text(search.kind),
  };
}

/** Name or id search, as Render's section search matches. */
export function matchesResourceQuery(
  row: Pick<ResourceRow, "id" | "name">,
  query: string,
): boolean {
  const needle = query.trim().toLocaleLowerCase();
  return (
    !needle ||
    row.name.toLocaleLowerCase().includes(needle) ||
    row.id.toLocaleLowerCase().includes(needle)
  );
}

export interface ProjectResourceCounts {
  all: number;
  services: number;
  databases: number;
  keyvalues: number;
  envgroups: number;
  sandboxgroups: number;
}

// Which count each row kind adds to. A sandbox group (w22/m603) has a count of
// its own, which Render's Services category includes. Sandbox groups are no
// family: their placement is read from the groups themselves, and a workspace
// whose bex-api has no sandboxes cannot read them at all, so an unread groups
// list is never shown as unknown rows.
const FILTER_KIND_BY_RESOURCE: Record<
  ResourceRow["kind"],
  Exclude<keyof ProjectResourceCounts, "all">
> = {
  service: "services",
  database: "databases",
  keyvalue: "keyvalues",
  envgroup: "envgroups",
  sandboxgroup: "sandboxgroups",
};

/** Counts are scoped to one environment's members and intentionally pre-search. */
export function countProjectResources(
  rows: ResourceRow[],
): ProjectResourceCounts {
  const counts: ProjectResourceCounts = {
    all: rows.length,
    services: 0,
    databases: 0,
    keyvalues: 0,
    envgroups: 0,
    sandboxgroups: 0,
  };
  for (const row of rows) {
    counts[FILTER_KIND_BY_RESOURCE[row.kind]] += 1;
  }
  return counts;
}

/**
 * Whether each family's list has answered (w4/m227). Membership names ids;
 * only these lists turn them into rows, so an unanswered or failed family's
 * rows are unknown, never zero.
 */
export interface ProjectResourceCatalog {
  /** Families whose list has not answered yet. */
  pending: ProjectResourceFamily[];
  /** Families whose list read failed. */
  failed: ProjectResourceFamily[];
  /** Re-reads the failed lists only. */
  retry: () => Promise<unknown>;
}

/** Counts as displayed: `null` where the rows are unknown. */
export type ProjectResourceDisplayCounts = {
  [K in keyof ProjectResourceCounts]: number | null;
};

/** Marks unknown families, and therefore the total, as unknown. */
export function countsWithUnknown(
  counts: ProjectResourceCounts,
  unknown: readonly ProjectResourceFamily[],
): ProjectResourceDisplayCounts {
  if (unknown.length === 0) return counts;
  const display: ProjectResourceDisplayCounts = { ...counts, all: null };
  for (const family of unknown) display[family] = null;
  return display;
}

/**
 * The switcher's counts: Render's Services category counts services,
 * Postgres, Key Value and sandbox groups together, so it is unknown while any
 * of those families is.
 */
export function environmentFilterCounts(
  counts: ProjectResourceDisplayCounts,
): Record<EnvironmentResourceFilter, number | null> {
  const parts = [
    counts.services,
    counts.databases,
    counts.keyvalues,
    counts.sandboxgroups,
  ];
  const services = parts.some((part) => part === null)
    ? null
    : parts.reduce<number>((sum, part) => sum + (part ?? 0), 0);
  return { all: counts.all, services, envgroups: counts.envgroups };
}
