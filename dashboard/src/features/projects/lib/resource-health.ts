import { deriveStatus as deriveServiceStatus } from "@/features/services/lib/status";
import { deriveStatus as deriveDatabaseStatus } from "@/features/databases/lib/status";
import { deriveStatus as deriveKeyValueStatus } from "@/features/keyvalue/lib/status";
import type { ResourceRow } from "@/features/projects/types";

/**
 * Whether a resource row is in a healthy, unremarkable state — the "is this
 * project card green or not" signal for the Overview page's project grid
 * (Render parity: the workspace homepage's project cards show a rolled-up
 * "All services are up and running" pill). A service counts as healthy while
 * suspended (an intentional, not a broken, state); a database/key-value
 * counts as healthy only while available and not suspended, mirroring what
 * their own status badges render as a non-alarming color.
 */
export function isRowHealthy(row: ResourceRow): boolean {
  if (row.kind === "service" && row.service) {
    const key = deriveServiceStatus(row.service).key;
    return key === "running" || key === "suspended" || key === "sleeping";
  }
  if (row.kind === "database" && row.database) {
    return (
      deriveDatabaseStatus(row.database).key === "available" &&
      row.database.suspended !== "suspended"
    );
  }
  if (row.kind === "keyvalue" && row.keyValue) {
    return (
      deriveKeyValueStatus(row.keyValue).key === "available" &&
      !row.keyValue.suspended
    );
  }
  return true;
}
