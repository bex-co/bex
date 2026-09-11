// Hooks binding serverActions / deployActions to the exact workspace +
// service on screen (w6/m143). Snapshots refresh on the m144 access
// generation so a new identity/workspace/access context never gates on
// stale decisions.

import { useEffect } from "react";
import { useQuery } from "@apollo/client/react";
import type { DocumentNode } from "graphql";
import {
  DeployActionsDocument,
  ServerActionsDocument,
} from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  toResourceSnapshot,
  type ResourceActionSnapshot,
} from "@/features/capabilities/lib/resource-actions";

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
  const { currentWorkspaceId } = useWorkspace();
  const capabilities = useCapabilities();
  const workspaceId = currentWorkspaceId;
  const query = useQuery(document, {
    variables,
    skip: !workspaceId || resourceId === null,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    notifyOnNetworkStatusChange: true,
  });

  // Access/identity/workspace transitions invalidate every projected decision.
  const generation = capabilities.generation;
  useEffect(() => {
    if (workspaceId && resourceId !== null) {
      void query.refetch(variables).catch(() => undefined);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [generation]);

  const refresh = async () => {
    try {
      await query.refetch(variables);
    } catch {
      // Fail closed: leave previous snapshot for display; next render re-derives.
    }
  };

  if (!workspaceId || resourceId === null) {
    return { status: "checking", refresh };
  }
  // Only a completed response for the CURRENT variables binds. Cached rows
  // from a previous target must never gate this target's controls.
  if (query.loading && select(query.data) === undefined) {
    return { status: "checking", refresh };
  }
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

/** Lifecycle decisions for one service (serverActions). */
export function useServerActions(
  serviceId: string | null,
): ResourceActionsState {
  return useProjection(
    ServerActionsDocument,
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
    DeployActionsDocument,
    { serviceId: serviceId ?? "" },
    selectField("deployActions"),
    serviceId,
  );
}
