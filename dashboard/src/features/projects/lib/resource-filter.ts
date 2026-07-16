import type { EnvGroupView } from "@/features/env-groups/types";
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
  envGroups: EnvGroupView[],
  filter: Pick<ProjectResourceFilterState, "query" | "kind">,
): { rows: ResourceRow[]; envGroups: EnvGroupView[] } {
  const query = filter.query.trim().toLocaleLowerCase();
  const matches = (name: string, id: string) =>
    !query ||
    name.toLocaleLowerCase().includes(query) ||
    id.toLocaleLowerCase().includes(query);
  const rowsForKind = rows.filter((row) => {
    if (filter.kind === "services") return row.kind === "service";
    if (filter.kind === "databases") return row.kind === "database";
    if (filter.kind === "keyvalues") return row.kind === "keyvalue";
    return filter.kind === "all";
  });
  return {
    rows: rowsForKind.filter((row) => matches(row.name, row.id)),
    envGroups:
      filter.kind === "all" || filter.kind === "envgroups"
        ? envGroups.filter((group) => matches(group.name, group.id))
        : [],
  };
}
