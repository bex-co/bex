import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  SuspendServiceDocument,
  ResumeServiceDocument,
  RestartServerDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import type { ServiceView, LifecycleAction } from "@/features/services/types";

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

export interface UseServiceLifecycleResult {
  pending: PendingLifecycle | null;
  run: (action: LifecycleAction, service: ServiceView) => Promise<void>;
}

const delay = (ms: number) =>
  new Promise((resolve) => setTimeout(resolve, ms));

// A service has converged once the observed state reflects the requested verb.
// suspend/resume flip `suspended`; restart rolls the pods and lands back on
// Running (the operator stamps a new template, Kubernetes rolls with no downtime).
const CONVERGED: Record<LifecycleAction, (s: ServiceView) => boolean> = {
  suspend: (s) => s.suspended,
  resume: (s) => !s.suspended,
  restart: (s) => !s.suspended && s.phase.toLowerCase() === "running",
};

const SUCCESS_KEY: Record<LifecycleAction, string> = {
  suspend: "services.toastSuspendSuccess",
  resume: "services.toastResumeSuccess",
  restart: "services.toastRestartSuccess",
};

/**
 * Wires the services list's row actions to bex-api's Render-named mutations
 * (`suspendService`/`resumeService`/`restartServer`). The patch is accepted
 * synchronously but the operator reconciles asynchronously, so after each verb
 * this polls the list until the row's observed state converges — the badge
 * reflects the operator, not an optimistic guess.
 */
export function useServiceLifecycle(
  opts: UseServiceLifecycleOptions,
): UseServiceLifecycleResult {
  const { refetch, pollIntervalMs = 2500, maxPolls = 12 } = opts;
  const { t } = useTranslations();
  const [pending, setPending] = useState<PendingLifecycle | null>(null);

  const [suspend] = useMutation(SuspendServiceDocument);
  const [resume] = useMutation(ResumeServiceDocument);
  const [restart] = useMutation(RestartServerDocument);
  const mutate: Record<LifecycleAction, (id: string) => Promise<unknown>> = {
    suspend: (id) => suspend({ variables: { id } }),
    resume: (id) => resume({ variables: { id } }),
    restart: (id) => restart({ variables: { id } }),
  };

  const pollUntilConverged = useCallback(
    async (action: LifecycleAction, id: string) => {
      const done = CONVERGED[action];
      for (let i = 0; i < maxPolls; i++) {
        await delay(pollIntervalMs);
        const list = await refetch();
        const svc = list.find((s) => s.id === id);
        if (svc && done(svc)) return;
      }
    },
    [refetch, pollIntervalMs, maxPolls],
  );

  const run = useCallback(
    async (action: LifecycleAction, service: ServiceView) => {
      setPending({ id: service.id, action });
      try {
        await mutate[action](service.id);
        toast.success(t(SUCCESS_KEY[action], { name: service.name }));
        await pollUntilConverged(action, service.id);
      } catch {
        toast.error(t("services.toastError", { name: service.name }));
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
