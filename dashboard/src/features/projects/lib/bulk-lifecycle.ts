// Render's service-list Suspend and Resume (w4/m550), as its dashboard bundle
// has them: docs/render-artifacts/service-list-bulk-actions.md.

import { isSuspended as isDatabaseSuspended } from "@/features/databases/lib/status";
import type { Capabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  allowsAction,
  confirmedDenied,
  grantReasonKey,
  type CapabilityState,
} from "@/features/capabilities/lib/capability-policy";
import {
  resourceSelectionKey,
  type ResourceRow,
} from "@/features/projects/types";

export type BulkLifecycleAction = "suspend" | "resume";

/** A selected row Suspend and Resume can act on, with the state it was in. */
export interface LifecycleTarget {
  key: string;
  kind: "service" | "database" | "keyvalue";
  id: string;
  name: string;
  suspended: boolean;
}

/** Render's batch width: at most 10 calls in flight (`Math.min(10, n)`). */
export const BULK_CONCURRENCY = 10;

/**
 * The selected rows Suspend and Resume act on. Render's rule is "anything but
 * a workflow": services of every type, Postgres and Key Value. bex has no
 * workflows, and an environment group has no lifecycle.
 */
export function lifecycleTargets(rows: ResourceRow[]): LifecycleTarget[] {
  return rows.flatMap((row): LifecycleTarget[] => {
    const base = { key: resourceSelectionKey(row), id: row.id, name: row.name };
    if (row.kind === "service" && row.service)
      return [{ ...base, kind: "service", suspended: row.service.suspended }];
    if (row.kind === "database" && row.database)
      return [
        {
          ...base,
          kind: "database",
          suspended: isDatabaseSuspended(row.database),
        },
      ];
    if (row.kind === "keyvalue" && row.keyValue)
      return [{ ...base, kind: "keyvalue", suspended: row.keyValue.suspended }];
    return [];
  });
}

/**
 * The targets an action changes: Suspend lists the running ones and Resume the
 * suspended ones. Render leaves the others out of its dialog rather than
 * reporting them, so a mixed selection offers both actions.
 */
export function targetsFor(
  action: BulkLifecycleAction,
  targets: LifecycleTarget[],
): LifecycleTarget[] {
  return targets.filter((target) =>
    action === "suspend" ? !target.suspended : target.suspended,
  );
}

function counted(count: number, noun: string): string {
  return `${count} ${noun}${count === 1 ? "" : "s"}`;
}

/**
 * Render's typed confirmation for a bulk Suspend: `sudo suspend 2 services`,
 * or `sudo suspend 2 services and 1 database` when Postgres is selected. A Key
 * Value instance counts as a service. A command, so never translated.
 */
export function suspendPhrase(targets: LifecycleTarget[]): string {
  const databases = targets.filter((t) => t.kind === "database").length;
  const services = targets.length - databases;
  const parts = [
    ...(services > 0 ? [counted(services, "service")] : []),
    ...(databases > 0 ? [counted(databases, "database")] : []),
  ];
  return `sudo suspend ${parts.join(" and ")}`;
}

/**
 * Why Suspend and Resume are disabled for this selection, or undefined when
 * they may run. Render disables the action, never drops rows from it, when any
 * selected resource refuses. bex's refusals are the workspace's: a role that
 * cannot operate refuses everywhere, and in a protected environment only an
 * Admin may suspend or resume (render.com/docs/projects; bex-api enforces the
 * same rule, w8/187), with Render's tooltip copy.
 */
export function bulkLifecycleReason(
  capabilities: Pick<
    Capabilities,
    "canOperate" | "canManage" | "denied" | "reasonKey"
  >,
  protectedEnvironment: boolean,
  t: (key: string) => string,
): string | undefined {
  const operateKey = capabilities.reasonKey("can_operate");
  if (operateKey) return t(operateKey);
  if (!capabilities.canOperate) return t("capabilities.actionChecking");
  if (!protectedEnvironment) return undefined;
  if (capabilities.denied("can_manage")) return t("capabilities.actionAdminOnly");
  const manageKey = capabilities.reasonKey("can_manage");
  if (manageKey) return t(manageKey);
  return capabilities.canManage ? undefined : t("capabilities.actionChecking");
}

/**
 * A capability answer read just before dispatch, in the shape
 * `bulkLifecycleReason` takes, so the toolbar's rule and the dispatch recheck
 * are the same rule (the w6/m146 freshness rule).
 */
export function capabilitiesFromState(
  state: CapabilityState,
  workspaceId: string | null,
): Parameters<typeof bulkLifecycleReason>[0] {
  return {
    canOperate: allowsAction(state, workspaceId, "can_operate"),
    canManage: allowsAction(state, workspaceId, "can_manage"),
    denied: (action) => confirmedDenied(state, workspaceId, action),
    reasonKey: (action) => grantReasonKey(state, workspaceId, action),
  };
}

/**
 * Runs `work` over `items` with at most `limit` in flight, each item started
 * as soon as a slot frees: Render's queue of `Math.min(10, n)` workers.
 */
export async function runBounded<T>(
  items: readonly T[],
  limit: number,
  work: (item: T) => Promise<void>,
): Promise<void> {
  let next = 0;
  const worker = async () => {
    while (next < items.length) {
      const item = items[next++];
      await work(item);
    }
  };
  await Promise.all(
    Array.from({ length: Math.min(limit, items.length) }, worker),
  );
}
