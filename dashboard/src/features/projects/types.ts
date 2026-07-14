import type { ServiceView } from "@/features/services/types";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";

/** The three resource kinds a project can group (w1/m31 extension). */
export type ResourceKind = "service" | "database" | "keyvalue";

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
  service?: ServiceView;
  database?: DatabaseView;
  keyValue?: KeyValueView;
}

export interface ResourceGroup {
  id: string;
  name: string;
  rows: ResourceRow[];
}
