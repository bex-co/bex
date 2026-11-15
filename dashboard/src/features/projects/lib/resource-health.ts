import { deriveStatus as deriveServiceStatus } from "@/features/services/lib/status";
import { deriveStatus as deriveDatabaseStatus } from "@/features/databases/lib/status";
import { deriveStatus as deriveKeyValueStatus } from "@/features/keyvalue/lib/status";
import type { ResourceKind, ResourceRow } from "@/features/projects/types";

export type ResourceHealth = "healthy" | "converging" | "attention";

interface HealthRule {
  /** The row's derived status key, or undefined when its view is absent. */
  statusKey: (row: ResourceRow) => string | undefined;
  healthy: readonly string[];
  converging: readonly string[];
}

/**
 * The classification table, one entry per resource kind: which derived status
 * keys are healthy or still converging. Any other derived key needs attention.
 * A kind with no entry (an env group) never needs attention through this path.
 */
const HEALTH_RULES: Partial<Record<ResourceKind, HealthRule>> = {
  service: {
    statusKey: (row) =>
      row.service ? deriveServiceStatus(row.service).key : undefined,
    healthy: ["running", "suspended", "sleeping"],
    converging: ["building", "deploying", "pending"],
  },
  database: {
    statusKey: (row) =>
      row.database ? deriveDatabaseStatus(row.database).key : undefined,
    healthy: ["available"],
    converging: ["creating", "restarting", "upgrading"],
  },
  keyvalue: {
    statusKey: (row) =>
      row.keyValue ? deriveKeyValueStatus(row.keyValue).key : undefined,
    healthy: ["available"],
    converging: ["creating", "restarting"],
  },
};

/**
 * Classify a resource for the Overview project's rolled-up status. Provisioning
 * is deliberately separate from failure: a Building service or Creating
 * database is normal work in progress and must not produce the red "needs
 * attention" message while its detail row shows a neutral progress badge.
 */
export function classifyResourceHealth(row: ResourceRow): ResourceHealth {
  const rule = HEALTH_RULES[row.kind];
  // A row whose kind-specific view has not loaded yet is not a failure.
  if (!rule) return "healthy";
  const key = rule.statusKey(row);
  if (key === undefined) return "healthy";
  if (rule.healthy.includes(key)) return "healthy";
  if (rule.converging.includes(key)) return "converging";
  return "attention";
}
