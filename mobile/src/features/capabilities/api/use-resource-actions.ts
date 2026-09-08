// ADR087 (w6/m141): hooks binding the per-resource action projections to the
// exact workspace + target on screen. A snapshot answers only for the
// workspace selected when it was fetched and the resource it was asked about —
// consumers read through resourceDecision/isExecutable, which fail closed on
// any mismatch. Snapshots refresh on the m139 access generation: a new
// identity/workspace/access generation must never gate on stale decisions.

import { useEffect } from "react";
import { useQuery } from "@apollo/client/react";
import type { DocumentNode } from "graphql";
import {
  MobileDatabaseActionsDocument,
  MobileDeployActionsDocument,
  MobileKeyValueActionsDocument,
  MobileServerActionsDocument,
} from "@/generated-graphql";
import { useCapabilities } from "../capabilities-provider";
import { useWorkspace } from "../../workspaces/workspace-provider";
import {
  toResourceSnapshot,
  type ResourceActionSnapshot,
} from "../resource-actions";

export type ResourceActionsState =
  | { status: "checking"; refresh: () => Promise<void> }
  | { status: "unavailable"; refresh: () => Promise<void> }
  | {
      status: "ready";
      snapshot: ResourceActionSnapshot;
      refresh: () => Promise<void>;
    };

type RawRow = {
  action?: string | null;
  outcome?: string | null;
  reason?: string | null;
  precondition?: string | null;
};

function useProjection(
  document: DocumentNode,
  variables: Record<string, string>,
  select: (data: unknown) => readonly RawRow[] | null | undefined,
  resourceId: string | null,
): ResourceActionsState {
  const { selected } = useWorkspace();
  const capabilities = useCapabilities();
  const workspaceId = selected?.id ?? null;
  const query = useQuery(document, {
    variables,
    skip: workspaceId === null || resourceId === null,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    notifyOnNetworkStatusChange: true,
  });

  // m139 dispatch gate: access/identity/workspace transitions invalidate
  // every projected decision. Refetch for the new generation; the snapshot
  // below stays bound to the previous workspace until the fresh answer lands.
  const generation = capabilities.generation;
  useEffect(() => {
    if (workspaceId !== null && resourceId !== null) {
      void query.refetch(variables).catch(() => undefined);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [generation]);

  const refresh = async () => {
    try {
      await query.refetch(variables);
    } catch {
      // Fail closed: a failed recheck leaves the previous snapshot in place
      // for display, but callers refresh alongside their data queries, so the
      // next render re-derives from current server state regardless.
    }
  };
  if (workspaceId === null || resourceId === null)
    return { status: "checking", refresh };
  // Only a completed response for the CURRENT variables binds. Cached rows
  // from a previous target (or a previous workspace) in flight must never
  // gate this target's controls — fail closed until the fresh answer lands.
  if (query.loading) return { status: "checking", refresh };
  if (query.error && select(query.data) === undefined) {
    return { status: "unavailable", refresh };
  }
  const rows = select(query.data);
  if (rows) {
    return {
      status: "ready",
      refresh,
      snapshot: toResourceSnapshot(
        workspaceId,
        resourceId,
        rows.flatMap((row) =>
          row.action && row.outcome
            ? [
                {
                  action: row.action,
                  outcome: row.outcome,
                  reason: row.reason ?? null,
                  precondition: row.precondition ?? null,
                },
              ]
            : [],
        ),
      ),
    };
  }
  if (query.error) return { status: "unavailable", refresh };
  return { status: "checking", refresh };
}

function selectField(field: string) {
  return (data: unknown): readonly RawRow[] | null | undefined => {
    if (!data || typeof data !== "object") return undefined;
    const rows = (data as Record<string, unknown>)[field];
    return Array.isArray(rows) ? (rows as RawRow[]) : undefined;
  };
}

/** Lifecycle + cron decisions for one service (serverActions). */
export function useServerActions(
  serviceId: string | null,
): ResourceActionsState {
  return useProjection(
    MobileServerActionsDocument,
    { id: serviceId ?? "" },
    selectField("serverActions"),
    serviceId,
  );
}

/** Deploy trigger/cancel/rollback decisions for one service. */
export function useDeployActions(
  serviceId: string | null,
): ResourceActionsState {
  return useProjection(
    MobileDeployActionsDocument,
    { serviceId: serviceId ?? "" },
    selectField("deployActions"),
    serviceId,
  );
}

/** Lifecycle decisions for one Postgres database. */
export function useDatabaseActions(
  databaseId: string | null,
): ResourceActionsState {
  return useProjection(
    MobileDatabaseActionsDocument,
    { id: databaseId ?? "" },
    selectField("databaseActions"),
    databaseId,
  );
}

/** Lifecycle decisions for one Key Value store. */
export function useKeyValueActions(
  storeId: string | null,
): ResourceActionsState {
  return useProjection(
    MobileKeyValueActionsDocument,
    { id: storeId ?? "" },
    selectField("keyValueActions"),
    storeId,
  );
}
