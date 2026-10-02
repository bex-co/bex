// Recheck a confirmation against its original context immediately before dispatch.

import { useCallback, useEffect, useRef, useState } from "react";
import { useApolloClient } from "@apollo/client/react";
import { toast } from "sonner";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  DeployActionsDocument,
  ServerActionsDocument,
  ViewerCapabilitiesDocument,
} from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { getAccessGeneration } from "@/features/capabilities/lib/access-generation";
import {
  allowsAction,
  toSnapshot,
  type CapabilityAction,
} from "@/features/capabilities/lib/capability-policy";
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
  requiredCapability?: CapabilityAction;
};

const SERVER_ACTIONS = new Set<ResourceActionId>([
  "restart",
  "suspend",
  "resume",
  "cron_run_now",
  "cron_cancel_run",
]);

export function useBoundActionConfirm(opts: {
  resourceId: string;
  deployId?: string;
  /** Additional fresh workspace grant needed by a more privileged variant. */
  requiredCapability?: CapabilityAction;
  /** Preserve the selected deploy's rollback eligibility when rechecking. */
  adjustDecision?: (
    action: ResourceActionId,
    decision: ResourceActionDecision | null,
  ) => ResourceActionDecision | null;
}) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const capabilities = useCapabilities();
  const { generation } = capabilities;
  const client = useApolloClient();
  const [pending, setPending] = useState<ActionConfirmBinding | null>(null);
  const pendingRef = useRef<ActionConfirmBinding | null>(null);
  const pendingAccessGeneration = useRef(getAccessGeneration());
  const mounted = useRef(true);
  const inFlight = useRef<AbortController | null>(null);
  const currentContext = {
    workspaceId: currentWorkspaceId,
    ...opts,
    generation,
    eligible:
      capabilities.loaded && !capabilities.stale && !capabilities.unavailable,
    accessGeneration: getAccessGeneration(),
  };
  const latest = useRef(currentContext);
  latest.current = currentContext;

  const isBindingCurrent = useCallback(
    (binding: ActionConfirmBinding | null) => {
      const current = latest.current;
      return (
        binding !== null &&
        mounted.current &&
        pendingRef.current === binding &&
        pendingAccessGeneration.current === getAccessGeneration() &&
        current.eligible &&
        current.workspaceId === binding.workspaceId &&
        current.resourceId === binding.resourceId &&
        current.deployId === binding.deployId &&
        current.generation === binding.generation &&
        current.requiredCapability === binding.requiredCapability &&
        current.accessGeneration === getAccessGeneration()
      );
    },
    [],
  );

  const clearConfirm = useCallback(() => {
    pendingRef.current = null;
    inFlight.current?.abort();
    setPending(null);
  }, []);

  const binding = isBindingCurrent(pending) ? pending : null;
  useEffect(() => {
    if (pending && !binding && pendingRef.current === pending) clearConfirm();
  }, [pending, binding, clearConfirm]);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      pendingRef.current = null;
      inFlight.current?.abort();
    };
  }, []);

  const openConfirm = useCallback(
    (action: ResourceActionId): ActionConfirmBinding | null => {
      const current = latest.current;
      if (
        !mounted.current ||
        !current.workspaceId ||
        !current.eligible ||
        current.accessGeneration !== getAccessGeneration()
      )
        return null;
      inFlight.current?.abort();
      const next: ActionConfirmBinding = {
        workspaceId: current.workspaceId,
        resourceId: current.resourceId,
        deployId: current.deployId,
        action,
        generation: current.generation,
        requiredCapability: current.requiredCapability,
      };
      pendingRef.current = next;
      pendingAccessGeneration.current = getAccessGeneration();
      setPending(next);
      return next;
    },
    [],
  );

  const recheckBeforeDispatch = useCallback(
    async (
      explicitBinding?: ActionConfirmBinding | null,
    ): Promise<{
      ok: boolean;
      binding: ActionConfirmBinding | null;
      precondition: string;
    }> => {
      const target =
        explicitBinding === undefined ? pendingRef.current : explicitBinding;
      const rejected = { ok: false, binding: null, precondition: "" };
      const refuse = (decision: ResourceActionDecision | null) => {
        // A response from a closed/replaced dialog must not close the new one,
        // toast on another page, or authorize its old mutation.
        if (!isBindingCurrent(target)) return rejected;
        clearConfirm();
        toast.error(
          gateReason(
            gateAction(decision, decision ? "ready" : "unavailable"),
            t,
          ),
        );
        return rejected;
      };
      if (!target || !isBindingCurrent(target)) return rejected;
      inFlight.current?.abort();
      const ac = new AbortController();
      inFlight.current = ac;
      const accessGeneration = getAccessGeneration();
      const current = () =>
        !ac.signal.aborted &&
        accessGeneration === getAccessGeneration() &&
        isBindingCurrent(target);
      const context = {
        fetchOptions: { signal: ac.signal },
        queryDeduplication: false,
      };
      try {
        const rows = SERVER_ACTIONS.has(target.action)
          ? (
              await client.query({
                query: ServerActionsDocument,
                variables: {
                  id: target.resourceId,
                  ownerId: target.workspaceId,
                },
                fetchPolicy: "no-cache",
                errorPolicy: "none",
                context,
              })
            ).data?.serverActions
          : (
              await client.query({
                query: DeployActionsDocument,
                variables: {
                  serviceId: target.resourceId,
                  ownerId: target.workspaceId,
                },
                fetchPolicy: "no-cache",
                errorPolicy: "none",
                context,
              })
            ).data?.deployActions;
        if (!current()) return rejected;
        if (!rows) return refuse(null);
        if (target.requiredCapability) {
          const result = await client.query({
            query: ViewerCapabilitiesDocument,
            variables: { ownerId: target.workspaceId, fresh: true },
            fetchPolicy: "no-cache",
            errorPolicy: "none",
            context,
          });
          if (!current()) return rejected;
          const payload = result.data?.viewerCapabilities;
          if (!payload) return refuse(null);
          const snapshot = toSnapshot(
            target.workspaceId,
            payload.grants ?? [],
            payload.role ?? null,
          );
          if (
            !allowsAction(
              { status: "ready", snapshot },
              target.workspaceId,
              target.requiredCapability,
            )
          ) {
            return refuse({
              action: target.action,
              outcome:
                snapshot.grants[target.requiredCapability] === "denied"
                  ? "denied"
                  : "unavailable",
              reason: "",
              precondition: "",
            });
          }
        }
        const snapshot = toResourceSnapshot(
          target.workspaceId,
          target.resourceId,
          rows,
        );
        const raw = resourceDecision(
          snapshot,
          target.workspaceId,
          target.resourceId,
          target.action,
        );
        const decision = latest.current.adjustDecision
          ? latest.current.adjustDecision(target.action, raw)
          : raw;
        if (!decision || gateAction(decision, "ready").kind !== "ready")
          return refuse(decision);
        if (!current()) return rejected;
        return {
          ok: true,
          binding: target,
          precondition: decision.precondition,
        };
      } catch {
        return current() ? refuse(null) : rejected;
      } finally {
        if (inFlight.current === ac) inFlight.current = null;
      }
    },
    [client, isBindingCurrent, clearConfirm, t],
  );

  return {
    pending: binding,
    openConfirm,
    clearConfirm,
    recheckBeforeDispatch,
    isBindingCurrent,
  };
}
