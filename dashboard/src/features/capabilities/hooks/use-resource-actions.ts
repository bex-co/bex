// Resource action results are short-lived evaluations bound to the exact
// workspace, resource and access generation that requested them.

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useApolloClient } from "@apollo/client/react";
import type { DocumentNode } from "graphql";
import {
  DeployActionsDocument,
  ServerActionsDocument,
} from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { getAccessGeneration } from "@/features/capabilities/lib/access-generation";
import { snapshotIsFresh } from "@/features/capabilities/lib/capability-policy";
import {
  toResourceSnapshot,
  type ResourceActionSnapshot,
  type ResourceActionInput,
} from "@/features/capabilities/lib/resource-actions";

export type ResourceActionsState =
  | { status: "checking"; refresh: () => Promise<void> }
  | { status: "unavailable"; refresh: () => Promise<void> }
  | {
      status: "ready";
      snapshot: ResourceActionSnapshot;
      refresh: () => Promise<void>;
    };

type ProjectionResult =
  | { status: "checking" | "unavailable" }
  | { status: "ready"; snapshot: ResourceActionSnapshot };

function useProjection(
  document: DocumentNode,
  field: "serverActions" | "deployActions",
  resourceId: string | null,
): ResourceActionsState {
  const { currentWorkspaceId: workspaceId } = useWorkspace();
  const capabilities = useCapabilities();
  const client = useApolloClient();
  const { generation, checkedAt } = capabilities;
  const eligible =
    capabilities.loaded && !capabilities.stale && !capabilities.unavailable;
  const lease = useMemo(
    () => ({ workspaceId, resourceId, generation, checkedAt, eligible }),
    [workspaceId, resourceId, generation, checkedAt, eligible],
  );
  // A layout effect, so the request effect below already sees this lease.
  const currentLease = useRef(lease);
  useLayoutEffect(() => {
    currentLease.current = lease;
  }, [lease]);
  const inFlight = useRef<AbortController | null>(null);
  const [result, setResult] = useState<{
    lease: typeof lease;
    value: ProjectionResult;
  } | null>(null);
  const settle = useCallback(
    (value: ProjectionResult) => setResult({ lease, value }),
    [lease],
  );

  // Answers through `onAnswer` only while this request is still the lease's
  // newest. `announce` withdraws the answer this same lease already holds; a
  // new lease has none and renders as checking until its first (see below).
  const evaluate = useCallback(
    async (
      onAnswer: (value: ProjectionResult) => void,
      { announce }: { announce: boolean },
    ) => {
      if (
        !workspaceId ||
        !resourceId ||
        !eligible ||
        currentLease.current !== lease
      )
        return;
      inFlight.current?.abort();
      const ac = new AbortController();
      inFlight.current = ac;
      const accessGeneration = getAccessGeneration();
      const current = () =>
        !ac.signal.aborted &&
        currentLease.current === lease &&
        getAccessGeneration() === accessGeneration;
      if (announce) onAnswer({ status: "checking" });
      try {
        const response = await client.query({
          query: document,
          variables:
            field === "serverActions"
              ? { id: resourceId, ownerId: workspaceId }
              : { serviceId: resourceId, ownerId: workspaceId },
          fetchPolicy: "no-cache",
          errorPolicy: "none",
          context: {
            fetchOptions: { signal: ac.signal },
            queryDeduplication: false,
          },
        });
        if (!current()) return;
        const rows = (
          response.data as Record<string, ResourceActionInput[]> | undefined
        )?.[field];
        if (!Array.isArray(rows)) throw new Error("action check unavailable");
        onAnswer({
          status: "ready",
          snapshot: toResourceSnapshot(workspaceId, resourceId, rows),
        });
      } catch {
        if (current()) onAnswer({ status: "unavailable" });
      } finally {
        if (inFlight.current === ac) inFlight.current = null;
      }
    },
    [client, document, field, workspaceId, resourceId, eligible, lease],
  );
  const refresh = useCallback(
    () => evaluate(settle, { announce: true }),
    [evaluate, settle],
  );

  // The provider's shared focus/reconnect/poll lifecycle supplies checkedAt.
  // Its expiry disables these results too, without adding one timer per row.
  useEffect(() => {
    void evaluate(settle, { announce: false });
    return () => inFlight.current?.abort();
  }, [evaluate, settle]);

  if (capabilities.unavailable || capabilities.stale)
    return { status: "unavailable", refresh };
  if (!eligible || result?.lease !== lease)
    return { status: "checking", refresh };
  if (
    result.value.status === "ready" &&
    !snapshotIsFresh(result.value.snapshot)
  )
    return { status: "unavailable", refresh };
  return { ...result.value, refresh };
}

export function useServerActions(
  serviceId: string | null,
): ResourceActionsState {
  return useProjection(ServerActionsDocument, "serverActions", serviceId);
}

export function useDeployActions(
  serviceId: string | null,
): ResourceActionsState {
  return useProjection(DeployActionsDocument, "deployActions", serviceId);
}
