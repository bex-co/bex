// Recheck a confirmation against its original context immediately before dispatch.
//
// Two predicates, kept apart on purpose (w4/m159). *Intent* — the open dialog,
// its target and any draft — belongs to an exact workspace/resource/deploy/
// access-generation context and survives a routine permission refresh.
// *Dispatch* additionally needs current, fresh, affirmative eligibility. A
// checking/stale/unavailable gap disables confirm and aborts any in-flight
// recheck, but never closes the dialog; recovery needs a new click.

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { useApolloClient } from "@apollo/client/react";
import { toast } from "sonner";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  queryProjection,
  type ProjectionField,
} from "@/features/capabilities/api/query-projection";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { getAccessGeneration } from "@/features/capabilities/lib/access-generation";
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

/** Which resource family's projection a recheck reads (w4/m578). */
export type ActionResourceKind = "service" | "database" | "keyValue";

function projectionFor(
  kind: ActionResourceKind,
  action: ResourceActionId,
): ProjectionField {
  if (kind === "database") return "databaseActions";
  if (kind === "keyValue") return "keyValueActions";
  return SERVER_ACTIONS.has(action) ? "serverActions" : "deployActions";
}

type IntentContext = {
  workspaceId: string | null;
  resourceId: string;
  deployId?: string;
  generation: number;
};

function sameIntent(binding: ActionConfirmBinding, context: IntentContext) {
  return (
    context.workspaceId === binding.workspaceId &&
    context.resourceId === binding.resourceId &&
    context.deployId === binding.deployId &&
    context.generation === binding.generation
  );
}

export function useBoundActionConfirm(opts: {
  resourceId: string;
  /** The resource's family; a service's by default. */
  kind?: ActionResourceKind;
  deployId?: string;
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
  const eligible =
    capabilities.loaded && !capabilities.stale && !capabilities.unavailable;
  const client = useApolloClient();
  const [pending, setPending] = useState<{
    binding: ActionConfirmBinding;
    accessGeneration: number;
  } | null>(null);
  const pendingRef = useRef<ActionConfirmBinding | null>(null);
  const pendingAccessGeneration = useRef(getAccessGeneration());
  const mounted = useRef(true);
  const inFlight = useRef<AbortController | null>(null);
  const currentContext = {
    workspaceId: currentWorkspaceId,
    ...opts,
    generation,
    eligible,
    accessGeneration: getAccessGeneration(),
  };
  const latest = useRef(currentContext);
  useLayoutEffect(() => {
    latest.current = currentContext;
  });

  /** Same-context intent: the dialog may stay open (presentation only). */
  const isIntentCurrent = useCallback(
    (binding: ActionConfirmBinding | null) => {
      const current = latest.current;
      return (
        binding !== null &&
        mounted.current &&
        pendingRef.current === binding &&
        pendingAccessGeneration.current === getAccessGeneration() &&
        sameIntent(binding, current) &&
        current.accessGeneration === getAccessGeneration()
      );
    },
    [],
  );

  /**
   * After a dispatched mutation resolves: may its result still be announced?
   * While mounted this is isIntentCurrent. A control the result itself
   * unmounts (a canceled deploy's row loses its Cancel, w4/623) still
   * announces it if no workspace switch or access change happened since
   * dispatch (both bump the access generation) and the current path (read
   * from the caller's router) still names the resource; a result after the
   * user left must not toast elsewhere (w4/m159).
   */
  const isResultCurrent = useCallback(
    (
      binding: ActionConfirmBinding,
      dispatchedAccessGeneration: number,
      currentPath: () => string,
    ) => {
      if (mounted.current) return isIntentCurrent(binding);
      return (
        dispatchedAccessGeneration === getAccessGeneration() &&
        currentPath().split("/").includes(binding.resourceId)
      );
    },
    [isIntentCurrent],
  );

  /** Intent plus fresh eligibility: the only predicate that may dispatch. */
  const isBindingCurrent = useCallback(
    (binding: ActionConfirmBinding | null) =>
      isIntentCurrent(binding) && latest.current.eligible,
    [isIntentCurrent],
  );

  const clearConfirm = useCallback(() => {
    pendingRef.current = null;
    inFlight.current?.abort();
    setPending(null);
  }, []);

  // isIntentCurrent for render. Its ref checks are implied: `pending` is set
  // and cleared together with pendingRef, unmount included.
  const binding =
    pending !== null &&
    pending.accessGeneration === currentContext.accessGeneration &&
    sameIntent(pending.binding, currentContext)
      ? pending.binding
      : null;
  useEffect(() => {
    if (pending && !binding && pendingRef.current === pending.binding)
      clearConfirm();
  }, [pending, binding, clearConfirm]);
  // A lapse invalidates an in-flight authorization attempt, so a check that
  // straddles it cannot resume after recovery; the dialog itself stays.
  useEffect(() => {
    if (!eligible) inFlight.current?.abort();
  }, [eligible]);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      clearConfirm();
    };
  }, [clearConfirm]);

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
      };
      pendingRef.current = next;
      pendingAccessGeneration.current = getAccessGeneration();
      setPending({
        binding: next,
        accessGeneration: pendingAccessGeneration.current,
      });
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
      try {
        const rows = await queryProjection(
          client,
          projectionFor(latest.current.kind ?? "service", target.action),
          target.resourceId,
          target.workspaceId,
          ac.signal,
        );
        if (!current()) return rejected;
        if (!rows) return refuse(null);
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
    /** Why an open confirmation cannot dispatch right now, if it cannot. */
    blockedReason:
      binding && !eligible
        ? t(
            capabilities.unavailable
              ? "capabilities.grantUnavailable"
              : "capabilities.actionChecking",
          )
        : undefined,
    openConfirm,
    clearConfirm,
    recheckBeforeDispatch,
    isBindingCurrent,
    isIntentCurrent,
    isResultCurrent,
  };
}
