import { useMemo } from "react";
import type { ServiceView } from "@/features/services/types";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";
import type { ProjectView } from "@/features/projects/hooks/use-projects";
import type { ResourceGroup, ResourceRow } from "@/features/projects/types";

export interface UseGroupedResourcesArgs {
  projects: ProjectView[];
  services: ServiceView[];
  databases: DatabaseView[];
  keyValues: KeyValueView[];
}

export interface UseGroupedResourcesResult {
  groups: ResourceGroup[];
  ungrouped: ResourceRow[];
}

/** Normalize a service into a merged-table `ResourceRow` (shared with the environments feature). */
export function toServiceRow(s: ServiceView): ResourceRow {
  return { kind: "service", id: s.id, name: s.name, createdAt: s.createdAt, service: s };
}

/** Normalize a database into a merged-table `ResourceRow` (shared with the environments feature). */
export function toDatabaseRow(d: DatabaseView): ResourceRow {
  return { kind: "database", id: d.id, name: d.name, createdAt: d.createdAt, database: d };
}

/** Normalize a key-value instance into a merged-table `ResourceRow` (shared with the environments feature). */
export function toKeyValueRow(k: KeyValueView): ResourceRow {
  return { kind: "keyvalue", id: k.id, name: k.name, createdAt: k.createdAt, keyValue: k };
}

/**
 * Normalizes services + databases + key-value instances into one merged
 * resource list, grouped by project (Render parity: a project page shows
 * every resource type together, docs/render-artifacts — w1/m31 extension).
 * Generalizes the service-only grouping that used to live inline in
 * `routes/index.tsx` to all three resource kinds, joining on each project's
 * `serviceIds`/`databaseIds`/`keyValueIds`.
 */
export function useGroupedResources({
  projects,
  services,
  databases,
  keyValues,
}: UseGroupedResourcesArgs): UseGroupedResourcesResult {
  return useMemo(() => {
    const serviceById = new Map(services.map((s) => [s.id, s]));
    const databaseById = new Map(databases.map((d) => [d.id, d]));
    const keyValueById = new Map(keyValues.map((k) => [k.id, k]));
    const assignedServiceIds = new Set<string>();
    const assignedDatabaseIds = new Set<string>();
    const assignedKeyValueIds = new Set<string>();

    const groups: ResourceGroup[] = projects.map((p) => {
      const rows: ResourceRow[] = [];
      for (const id of p.serviceIds) {
        const s = serviceById.get(id);
        if (s) {
          rows.push(toServiceRow(s));
          assignedServiceIds.add(id);
        }
      }
      for (const id of p.databaseIds) {
        const d = databaseById.get(id);
        if (d) {
          rows.push(toDatabaseRow(d));
          assignedDatabaseIds.add(id);
        }
      }
      for (const id of p.keyValueIds) {
        const k = keyValueById.get(id);
        if (k) {
          rows.push(toKeyValueRow(k));
          assignedKeyValueIds.add(id);
        }
      }
      return { id: p.id, name: p.name, rows };
    });

    const ungrouped: ResourceRow[] = [
      ...services.filter((s) => !assignedServiceIds.has(s.id)).map(toServiceRow),
      ...databases.filter((d) => !assignedDatabaseIds.has(d.id)).map(toDatabaseRow),
      ...keyValues.filter((k) => !assignedKeyValueIds.has(k.id)).map(toKeyValueRow),
    ];

    return { groups, ungrouped };
  }, [projects, services, databases, keyValues]);
}
