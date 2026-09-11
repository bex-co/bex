// Bind and recheck an open confirmation against the current workspace,
// resource, optional deploy, and action decision before dispatch (w6/m143/t004).

import { useCallback, useState } from "react";
import { useApolloClient } from "@apollo/client/react";
import {
  DeployActionsDocument,
  ServerActionsDocument,
} from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  toResourceSnapshot,
  resourceDecision,
  type ResourceActionId,
} from "@/features/capabilities/lib/resource-actions";

export type ActionConfirmBinding = {
  workspaceId: string;
  resourceId: string;
  deployId?: string;
  action: ResourceActionId;
  generation: number;
};

const SERVER_ACTIONS = new Set<ResourceActionId>([
  "restart",
  "suspend",
  "resume",
  "cron_run_now",
  "cron_cancel_run",
]);

/**
 * Holds a pending confirmation bound to an exact context. Stale bindings are
 * derived away (not cleared in an effect) when workspace/resource/deploy or
 * access generation drifts; dispatch always rechecks over the network.
 */
export function useBoundActionConfirm(opts: {
  resourceId: string;
  deployId?: string;
}) {
  const { currentWorkspaceId } = useWorkspace();
  const { generation } = useCapabilities();
  const client = useApolloClient();
  const [pending, setPending] = useState<ActionConfirmBinding | null>(null);

  const binding =
    pending !== null &&
    currentWorkspaceId !== null &&
    pending.workspaceId === currentWorkspaceId &&
    pending.resourceId === opts.resourceId &&
    (pending.deployId === undefined || pending.deployId === opts.deployId) &&
    pending.generation === generation
      ? pending
      : null;

  const openConfirm = useCallback(
    (action: ResourceActionId) => {
      if (!currentWorkspaceId) return;
      setPending({
        workspaceId: currentWorkspaceId,
        resourceId: opts.resourceId,
        deployId: opts.deployId,
        action,
        generation,
      });
    },
    [currentWorkspaceId, opts.resourceId, opts.deployId, generation],
  );

  const clearConfirm = useCallback(() => {
    setPending(null);
  }, []);

  const recheckBeforeDispatch = useCallback(async (): Promise<{
    ok: boolean;
    binding: ActionConfirmBinding | null;
    precondition: string;
  }> => {
    if (!binding || !currentWorkspaceId) {
      setPending(null);
      return { ok: false, binding: null, precondition: "" };
    }

    try {
      const useServer = SERVER_ACTIONS.has(binding.action);
      let rows:
        | {
            action: string;
            outcome: string;
            reason?: string | null;
            precondition?: string | null;
          }[]
        | null
        | undefined;
      if (useServer) {
        const result = await client.query({
          query: ServerActionsDocument,
          variables: { id: binding.resourceId },
          fetchPolicy: "network-only",
          errorPolicy: "all",
        });
        rows = result.data?.serverActions;
      } else {
        const result = await client.query({
          query: DeployActionsDocument,
          variables: { serviceId: binding.resourceId },
          fetchPolicy: "network-only",
          errorPolicy: "all",
        });
        rows = result.data?.deployActions;
      }
      if (!rows) {
        setPending(null);
        return { ok: false, binding: null, precondition: "" };
      }
      const snapshot = toResourceSnapshot(
        binding.workspaceId,
        binding.resourceId,
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
      );
      const decision = resourceDecision(
        snapshot,
        binding.workspaceId,
        binding.resourceId,
        binding.action,
      );
      const ok =
        decision !== null &&
        decision.outcome === "allowed" &&
        (decision.precondition === "" ||
          decision.precondition === "protected_confirmation_required");
      if (!ok) {
        setPending(null);
        return { ok: false, binding: null, precondition: "" };
      }
      return {
        ok: true,
        binding,
        precondition: decision.precondition,
      };
    } catch {
      setPending(null);
      return { ok: false, binding: null, precondition: "" };
    }
  }, [binding, client, currentWorkspaceId]);

  return {
    pending: binding,
    openConfirm,
    clearConfirm,
    recheckBeforeDispatch,
  };
}
