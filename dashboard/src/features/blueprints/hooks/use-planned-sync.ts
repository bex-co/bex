import { useCallback, useMemo, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  ApproveBlueprintDocument,
  BlueprintPlannedSyncDocument,
  PlanBlueprintSyncDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";
import { toPlanActions } from "@/features/blueprints/lib/views";
import type { BlueprintPlanAction } from "@/features/blueprints/types";

export interface ManualEnvVarRequest {
  serviceType: string;
  serviceName: string;
  key: string;
}

export interface PlannedSyncView {
  id: string;
  commitId: string;
  /** Empty for a plan recorded before plans kept their actions. */
  actions: BlueprintPlanAction[];
  requiredManualEnvVars: ManualEnvVarRequest[];
}

export interface UsePlannedSyncResult {
  planned: PlannedSyncView | null;
  loading: boolean;
  busy: boolean;
  /** Applies the plan with values keyed by `${serviceName}/${key}`. */
  approve: (values: Record<string, string>) => Promise<boolean>;
  replan: () => Promise<void>;
}

/** manualEnvVarKey names one required value in the approve form. */
export function manualEnvVarKey(v: ManualEnvVarRequest): string {
  return `${v.serviceName}/${v.key}`;
}

/**
 * The plan a created Blueprint waits to have approved (w2/m256 t012), and the
 * two things that can happen to it: approve (apply it with its manual env var
 * values) or plan again. Render's plan-then-approve flow applies a created
 * Blueprint only this way; the legacy Sync refuses it.
 */
export function usePlannedSync(
  blueprintId: string,
  enabled: boolean,
): UsePlannedSyncResult {
  const { t } = useTranslations();
  const { data, loading } = useQuery(BlueprintPlannedSyncDocument, {
    variables: { id: blueprintId },
    skip: !enabled,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  // Approving moves the Blueprint off created, which unmounts the panel, so
  // only the page and its history are re-read.
  const [approveMutation] = useMutation(ApproveBlueprintDocument, {
    refetchQueries: ["Blueprint", "BlueprintSyncs"],
  });
  const [planMutation] = useMutation(PlanBlueprintSyncDocument, {
    refetchQueries: ["BlueprintPlannedSync", "BlueprintSyncs"],
  });
  const paymentGate = usePaymentRequiredGate();
  const [busy, setBusy] = useState(false);

  const planned = useMemo<PlannedSyncView | null>(() => {
    const p = data?.blueprintPlannedSync;
    if (!p?.id) return null;
    return {
      id: p.id,
      commitId: p.commitId ?? "",
      actions: toPlanActions(p.actions),
      requiredManualEnvVars: (p.requiredManualEnvVars ?? []).flatMap((v) =>
        v
          ? [
              {
                serviceType: v.serviceType ?? "",
                serviceName: v.serviceName ?? "",
                key: v.key ?? "",
              },
            ]
          : [],
      ),
    };
  }, [data]);

  const approve = useCallback(
    async (values: Record<string, string>) => {
      if (!planned) return false;
      setBusy(true);
      try {
        await paymentGate.run(() =>
          approveMutation({
            variables: {
              id: blueprintId,
              manualEnvVars: planned.requiredManualEnvVars.map((v) => ({
                ...v,
                value: values[manualEnvVarKey(v)] ?? "",
              })),
            },
          }),
        );
        toast.success(t("blueprints.plannedApproved"));
        return true;
      } catch (err) {
        if (isPaymentOnboardingCancelled(err)) return false;
        toast.error(
          mutationErrorMessage(err, t("blueprints.plannedApproveFailed")),
        );
        return false;
      } finally {
        setBusy(false);
      }
    },
    [approveMutation, blueprintId, paymentGate, planned, t],
  );

  const replan = useCallback(async () => {
    setBusy(true);
    try {
      await planMutation({ variables: { id: blueprintId } });
    } catch (err) {
      toast.error(
        mutationErrorMessage(err, t("blueprints.plannedReplanFailed")),
      );
    } finally {
      setBusy(false);
    }
  }, [planMutation, blueprintId, t]);

  return { planned, loading, busy, approve, replan };
}
