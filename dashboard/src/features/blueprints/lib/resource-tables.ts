import type { BlueprintResource } from "@/features/blueprints/types";

/** The service types `Blueprint.resources` names (`effectiveType`, bex-api). */
const SERVICE_TYPES = new Set([
  "web_service",
  "private_service",
  "background_worker",
  "cron_job",
  "static_site",
]);

/**
 * A Blueprint's resources in the tables Render's Resources page draws them in
 * (w3/m680): its services, Postgres and Key Value in the resource table the
 * Overview uses, its build sources in the Build Sources table, its environment
 * groups in the Environment Groups table. A type this build does not know
 * goes in none, as Render draws only the kinds it has a table for.
 */
export interface BlueprintResourceTables {
  resources: BlueprintResource[];
  buildSources: BlueprintResource[];
  envGroups: BlueprintResource[];
}

export function blueprintResourceTables(
  resources: readonly BlueprintResource[] | null,
): BlueprintResourceTables {
  const tables: BlueprintResourceTables = {
    resources: [],
    buildSources: [],
    envGroups: [],
  };
  for (const resource of resources ?? []) {
    if (
      SERVICE_TYPES.has(resource.type) ||
      resource.type === "postgres" ||
      resource.type === "key_value"
    ) {
      tables.resources.push(resource);
    } else if (resource.type === "build_source") {
      tables.buildSources.push(resource);
    } else if (resource.type === "environment_group") {
      tables.envGroups.push(resource);
    }
  }
  return tables;
}

/** How many rows each of the page's tables has, which its frame draws. */
export interface BlueprintResourcesShape {
  resources: number;
  buildSources: number;
  envGroups: number;
}

export function blueprintResourcesShape(
  tables: BlueprintResourceTables,
): BlueprintResourcesShape {
  return {
    resources: tables.resources.length,
    buildSources: tables.buildSources.length,
    envGroups: tables.envGroups.length,
  };
}

/** Whether a resource of `type` is one of the Blueprint's services. */
export function isBlueprintServiceType(type: string): boolean {
  return SERVICE_TYPES.has(type);
}
