import type { ResourceRow } from "@/features/projects/types";

export const PROJECT_RESOURCE_KINDS = [
  "all",
  "services",
  "databases",
  "keyvalues",
  "envgroups",
] as const;
export type ProjectResourceKind = (typeof PROJECT_RESOURCE_KINDS)[number];

const FILTER_KIND_BY_RESOURCE: Record<
  ResourceRow["kind"],
  Exclude<ProjectResourceKind, "all">
> = {
  service: "services",
  database: "databases",
  keyvalue: "keyvalues",
  envgroup: "envgroups",
};

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
  return rows.filter((row) => {
    if (
      filter.kind !== "all" &&
      FILTER_KIND_BY_RESOURCE[row.kind] !== filter.kind
    )
      return false;
    return (
      !query ||
      row.name.toLocaleLowerCase().includes(query) ||
      row.id.toLocaleLowerCase().includes(query)
    );
  });
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
    counts[FILTER_KIND_BY_RESOURCE[row.kind]] += 1;
  }
  return counts;
}
