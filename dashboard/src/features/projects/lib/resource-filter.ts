import type { ResourceRow } from "@/features/projects/types";

export const PROJECT_RESOURCE_KINDS = [
  "all",
  "services",
  "databases",
  "keyvalues",
  "envgroups",
] as const;
export type ProjectResourceKind = (typeof PROJECT_RESOURCE_KINDS)[number];

export interface ProjectResourceFilterState {
  environmentId: string | null;
  query: string;
  kind: ProjectResourceKind;
}

export interface ProjectResourceSearch {
  env?: string;
  q?: string;
  kind?: Exclude<ProjectResourceKind, "all">;
}

export function parseProjectResourceKind(value: unknown): ProjectResourceKind {
  return typeof value === "string" &&
    PROJECT_RESOURCE_KINDS.includes(value as ProjectResourceKind)
    ? (value as ProjectResourceKind)
    : "all";
}

/** Sanitizes the project route's shareable environment/search/type state. */
export function parseProjectResourceSearch(
  search: Record<string, unknown>,
): ProjectResourceSearch {
  const kind = parseProjectResourceKind(search.kind);
  return {
    ...(typeof search.env === "string" && search.env
      ? { env: search.env }
      : {}),
    ...(typeof search.q === "string" && search.q ? { q: search.q } : {}),
    ...(kind !== "all" ? { kind } : {}),
  };
}

/** Composes name/id search with the selected kind over one environment's members. */
export function filterProjectResources(
  rows: ResourceRow[],
  filter: Pick<ProjectResourceFilterState, "query" | "kind">,
): ResourceRow[] {
  const query = filter.query.trim().toLocaleLowerCase();
  const matches = (name: string, id: string) =>
    !query ||
    name.toLocaleLowerCase().includes(query) ||
    id.toLocaleLowerCase().includes(query);
  const rowsForKind = rows.filter((row) => {
    if (filter.kind === "services") return row.kind === "service";
    if (filter.kind === "databases") return row.kind === "database";
    if (filter.kind === "keyvalues") return row.kind === "keyvalue";
    if (filter.kind === "envgroups") return row.kind === "envgroup";
    return filter.kind === "all";
  });
  return rowsForKind.filter((row) => matches(row.name, row.id));
}

export interface ProjectResourceCounts {
  all: number;
  services: number;
  databases: number;
  keyvalues: number;
  envgroups: number;
}

/** Counts are scoped to the selected Environment and intentionally pre-search. */
export function countProjectResources(
  rows: ResourceRow[],
): ProjectResourceCounts {
  const counts: ProjectResourceCounts = {
    all: rows.length,
    services: 0,
    databases: 0,
    keyvalues: 0,
    envgroups: 0,
  };
  for (const row of rows) {
    if (row.kind === "service") counts.services += 1;
    if (row.kind === "database") counts.databases += 1;
    if (row.kind === "keyvalue") counts.keyvalues += 1;
    if (row.kind === "envgroup") counts.envgroups += 1;
  }
  return counts;
}
