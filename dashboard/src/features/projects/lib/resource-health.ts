import { deriveStatus as deriveServiceStatus } from "@/features/services/lib/status";
import { deriveStatus as deriveDatabaseStatus } from "@/features/databases/lib/status";
import { deriveStatus as deriveKeyValueStatus } from "@/features/keyvalue/lib/status";
import type { ResourceRow } from "@/features/projects/types";

export type ResourceHealth = "healthy" | "converging" | "attention";

/**
 * Classify a resource for the Overview project's rolled-up status. Provisioning
 * is deliberately separate from failure: a Building service or Creating
 * database is normal work in progress and must not produce the red "needs
 * attention" message while its detail row shows a neutral progress badge.
 */
export function classifyResourceHealth(row: ResourceRow): ResourceHealth {
  if (row.kind === "service" && row.service) {
    const key = deriveServiceStatus(row.service).key;
    if (key === "running" || key === "suspended" || key === "sleeping") {
      return "healthy";
    }
    if (key === "building" || key === "deploying" || key === "pending") {
      return "converging";
    }
    return "attention";
  }
  if (row.kind === "database" && row.database) {
    const key = deriveDatabaseStatus(row.database).key;
    if (key === "available") return "healthy";
    if (key === "creating" || key === "upgrading") return "converging";
    return "attention";
  }
  if (row.kind === "keyvalue" && row.keyValue) {
    const key = deriveKeyValueStatus(row.keyValue).key;
    if (key === "available") return "healthy";
    if (key === "creating") return "converging";
    return "attention";
  }
  return "healthy";
}
