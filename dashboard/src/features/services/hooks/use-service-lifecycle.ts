import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  SuspendServiceDocument,
  ResumeServiceDocument,
  TriggerDeployDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { sleep } from "@/common/lib/utils/time";
import { deriveStatus } from "@/features/services/lib/status";
import type { ServiceView, LifecycleAction } from "@/features/services/types";
import {
  protectedConfirmationFromError,
  type ProtectedActionResult,
} from "@/features/services/lib/protected-confirmation";

/** The action currently in flight for a given service id (or null). */
export interface PendingLifecycle {
  id: string;
  action: LifecycleAction;
}

export interface UseServiceLifecycleOptions {
  /** Re-runs the services query and resolves the fresh list; used to poll. */
  refetch: () => Promise<ServiceView[]>;
  /** Poll cadence while waiting for the operator to converge. Default 2.5s. */
  pollIntervalMs?: number;
  /** Max polls before giving up (the UI still reflects the latest state). Default 12. */
  maxPolls?: number;
}

export type RunServiceAction = (
  action: LifecycleAction,
  service: ServiceView,
  confirmation?: string,
) => Promise<ProtectedActionResult>;

export interface UseServiceLifecycleResult {
  pending: PendingLifecycle | null;
  run: RunServiceAction;
}

// A service has converged once the observed state reflects the requested verb.
// suspend/resume flip `suspended`; restart rolls the pods and lands back on
// Running (the operator stamps a new template, Kubernetes rolls with no downtime).
// "running" reuses the sealed derivation (`deriveStatus`) so the phase-casing
// rule stays in lib/status.ts, not re-spelled here.
const CONVERGED: Record<LifecycleAction, (s: ServiceView) => boolean> = {
  suspend: (s) => s.suspended,
  resume: (s) => !s.suspended,
  restart: (s) => deriveStatus(s).key === "running",
};

const SUCCESS_KEY: Record<LifecycleAction, string> = {
  suspend: "services.toastSuspendSuccess",
  resume: "services.toastResumeSuccess",
  restart: "services.toastRestartSuccess",
};

/**
 * Wires the services list's row actions to bex-api's Render-named mutations
 * (`suspendService`/`resumeService`/`triggerDeploy` for restart). The patch is
 * accepted synchronously but the operator reconciles asynchronously, so after
 * each verb this polls the list until the row's observed state converges —
 * the badge reflects the operator, not an optimistic guess.
 *
 * Restart is routed through `triggerDeploy` (w2/m30 consolidation) so it
 * always opens a deploy-history row in the Events tab, same as every other
 * rollout. For repo-backed services this triggers a rebuild from Branch HEAD;
 * for image-backed services it re-pulls and restarts the containers.
 */
export function useServiceLifecycle(
  opts: UseServiceLifecycleOptions,
): UseServiceLifecycleResult {
  const { refetch, pollIntervalMs = 2500, maxPolls = 12 } = opts;
  const { t } = useTranslations();
  const [pending, setPending] = useState<PendingLifecycle | null>(null);

  const [suspend] = useMutation(SuspendServiceDocument);
  const [resume] = useMutation(ResumeServiceDocument);
  const [triggerDeploy] = useMutation(TriggerDeployDocument, {
    refetchQueries: ["ServiceEvents"],
  });
  const mutate: Record<
    LifecycleAction,
    (id: string, confirmation?: string) => Promise<unknown>
  > = {
    suspend: (id, confirmation) =>
      suspend({ variables: { id, confirm: confirmation } }),
    resume: (id) => resume({ variables: { id } }),
    restart: (id) => triggerDeploy({ variables: { serviceId: id } }),
  };

  const pollUntilConverged = useCallback(
    async (action: LifecycleAction, id: string) => {
      const done = CONVERGED[action];
      for (let i = 0; i < maxPolls; i++) {
        await sleep(pollIntervalMs);
        const list = await refetch();
        const svc = list.find((s) => s.id === id);
        if (svc && done(svc)) return;
      }
    },
    [refetch, pollIntervalMs, maxPolls],
  );

  const run = useCallback(
    async (
      action: LifecycleAction,
      service: ServiceView,
      confirmation?: string,
    ) => {
      setPending({ id: service.id, action });
      try {
        await mutate[action](service.id, confirmation);
        toast.success(t(SUCCESS_KEY[action], { name: service.name }));
        await pollUntilConverged(action, service.id);
        return { status: "success" } as const;
      } catch (err) {
        const requiredConfirmation = protectedConfirmationFromError(err);
        if (requiredConfirmation) {
          return {
            status: "confirmation_required",
            confirmation: requiredConfirmation,
          } as const;
        }
        toast.error(t("services.toastError", { name: service.name }));
        return { status: "error" } as const;
      } finally {
        setPending(null);
      }
    },
    // mutate closes over the three stable mutate fns; t/pollUntilConverged are stable.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, pollUntilConverged],
  );

  return { pending, run };
}
