import { useCallback, useMemo, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import {
  ApproveBlueprintDocument,
  BlueprintSyncDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { skipPollWhenHidden, useConvergingPoll } from "@/common/lib/polling";
import {
  hasGraphQLErrorCode,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";
import { toBlueprintSyncDetailView } from "@/features/blueprints/lib/views";
import {
  isApplyingSyncState,
  isSettledSyncState,
} from "@/features/blueprints/lib/sync-state";
import type {
  BlueprintManualEnvVar,
  BlueprintSyncDetailView,
} from "@/features/blueprints/types";

/** manualEnvVarKey names one required value in the Approve form. */
export function manualEnvVarKey(v: BlueprintManualEnvVar): string {
  return `${v.serviceName}/${v.key}`;
}

export interface UseBlueprintSyncResult {
  sync: BlueprintSyncDetailView | null;
  loading: boolean;
  error: Error | undefined;
  refetch: () => void;
  approving: boolean;
  /** The last Approve's failure, Render's inline alert; null otherwise. */
  approveError: string | null;
  /** Applies the waiting plan with values keyed by `manualEnvVarKey`. */
  approve: (values: Record<string, string>) => Promise<void>;
}

/**
 * One sync of a Blueprint for its page (w3/m678), Render's iacExecution
 * query: read until it settles, fast while it applies, so the page follows a
 * sync to its end without a reload (Render subscribes until success or
 * error). Approve applies a waiting plan, bound to this sync's id so a plan
 * made since is refused, not applied (w5/m384).
 */
export function useBlueprintSync(
  blueprintId: string,
  syncId: string,
): UseBlueprintSyncResult {
  const { t } = useTranslations();
  const { data, loading, error, refetch, startPolling, stopPolling } = useQuery(
    BlueprintSyncDocument,
    {
      variables: { id: blueprintId, syncId },
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      skipPollAttempt: skipPollWhenHidden,
    },
  );
  const sync = useMemo(
    () =>
      data?.blueprintSync?.id
        ? toBlueprintSyncDetailView(data.blueprintSync)
        : null,
    [data],
  );
  useConvergingPoll(
    startPolling,
    stopPolling,
    !!sync && isApplyingSyncState(sync.state),
    !!sync && !isSettledSyncState(sync.state),
  );

  // Approving applies the plan: the Blueprint's status, its history and this
  // sync all move, so each is read again.
  const [approveMutation] = useMutation(ApproveBlueprintDocument, {
    refetchQueries: ["BlueprintSync", "Blueprint", "BlueprintSyncs"],
  });
  const paymentGate = usePaymentRequiredGate();
  const [approving, setApproving] = useState(false);
  const [approveError, setApproveError] = useState<string | null>(null);

  const approve = useCallback(
    async (values: Record<string, string>) => {
      if (!sync) return;
      setApproving(true);
      setApproveError(null);
      try {
        await paymentGate.run(() =>
          approveMutation({
            variables: {
              id: blueprintId,
              syncId: sync.id,
              manualEnvVars: sync.requiredManualEnvVars.map((v) => ({
                ...v,
                value: values[manualEnvVarKey(v)] ?? "",
              })),
            },
          }),
        );
      } catch (err) {
        if (isPaymentOnboardingCancelled(err)) return;
        setApproveError(
          hasGraphQLErrorCode(err, "BLUEPRINT_PLAN_CHANGED")
            ? t("blueprints.plannedChanged")
            : mutationErrorMessage(err, t("blueprints.syncApproveFailed")),
        );
      } finally {
        setApproving(false);
      }
    },
    [approveMutation, blueprintId, paymentGate, sync, t],
  );

  return {
    sync,
    loading: loading && !sync,
    error,
    refetch: () => void refetch().catch(() => undefined),
    approving,
    approveError,
    approve,
  };
}
