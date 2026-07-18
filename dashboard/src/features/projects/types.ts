import type { ServiceView } from "@/features/services/types";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";
import type { EnvGroupView } from "@/features/env-groups/types";

/** Every resource kind rendered in an Environment's unified operating table. */
export type ResourceKind = "service" | "database" | "keyvalue" | "envgroup";

/**
 * One row in the unified Projects page's merged table — a service, database,
 * or key-value instance normalized to the same shape so they can share one
 * `<Table>` with a Type column. `service`/`database`/`keyValue` carries the
 * original view (only the one matching `kind` is set) for the kind-specific
 * status badge + row-actions component.
 */
export interface ResourceRow {
  kind: ResourceKind;
  id: string;
  name: string;
  createdAt: string | null;
  /** Authoritative resource mutation time. Never substituted with createdAt. */
  updatedAt: string | null;
  /** Server-backed runtime/product label; null means the fact is unavailable. */
  runtime: string | null;
  /** Explicit installation placement; null means the fact is unavailable. */
  region: string | null;
  service?: ServiceView;
  database?: DatabaseView;
  keyValue?: KeyValueView;
  envGroup?: EnvGroupView;
}

/** Stable selection identity across filters and resource kinds. */
export function resourceSelectionKey(
  row: Pick<ResourceRow, "kind" | "id">,
): string {
  return `${row.kind}:${row.id}`;
}

export interface ResourceGroup {
  id: string;
  name: string;
  rows: ResourceRow[];
}
