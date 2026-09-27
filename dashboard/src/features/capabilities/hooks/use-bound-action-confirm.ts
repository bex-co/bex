// Bind and recheck an open confirmation against the current workspace,
// resource, optional deploy, and action decision before dispatch (w6/m143/t004).

import { useCallback, useState } from "react";
import { useApolloClient } from "@apollo/client/react";
import { toast } from "sonner";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  DeployActionsDocument,
  ServerActionsDocument,
} from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  gateAction,
  gateReason,
  toResourceSnapshot,
  resourceDecision,
  type ResourceActionDecision,
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
 *
 * `adjustDecision` is the caller's row-level rule, applied to the rechecked
 * decision exactly as it was applied when enabling the control. Without it,
 * a row the button enabled (an older rollback target the service-wide summary
 * does not see) was refused at dispatch (w4/m141). A refusal is never silent:
 * it is toasted with its reason before the recheck resolves `ok: false`.
 */
export function useBoundActionConfirm(opts: {
  resourceId: string;
  deployId?: string;
  adjustDecision?: (
    action: ResourceActionId,
    decision: ResourceActionDecision | null,
  ) => ResourceActionDecision | null;
}) {
  const { t } = useTranslations();
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

  const { adjustDecision } = opts;
  const recheckBeforeDispatch = useCallback(async (): Promise<{
    ok: boolean;
    binding: ActionConfirmBinding | null;
    precondition: string;
  }> => {
    // Every refusal says why: a dispatch that quietly does nothing reads as
    // success to someone who just clicked Proceed (w4/m141).
    const refuse = (decision: ResourceActionDecision | null) => {
      setPending(null);
      toast.error(
        gateReason(gateAction(decision, decision ? "ready" : "unavailable"), t),
      );
      return { ok: false, binding: null, precondition: "" };
    };
    if (!binding || !currentWorkspaceId) {
      return refuse(null);
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
        return refuse(null);
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
      const raw = resourceDecision(
        snapshot,
        binding.workspaceId,
        binding.resourceId,
        binding.action,
      );
      const decision = adjustDecision
        ? adjustDecision(binding.action, raw)
        : raw;
      const ok =
        decision !== null &&
        decision.outcome === "allowed" &&
        (decision.precondition === "" ||
          decision.precondition === "protected_confirmation_required");
      if (!ok) {
        return refuse(decision);
      }
      return {
        ok: true,
        binding,
        precondition: decision.precondition,
      };
    } catch {
      return refuse(null);
    }
  }, [binding, client, currentWorkspaceId, adjustDecision, t]);

  return {
    pending: binding,
    openConfirm,
    clearConfirm,
    recheckBeforeDispatch,
  };
}
