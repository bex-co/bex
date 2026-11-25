import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateServicePlanDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";

/** A plan change the server accepted. */
export interface SavedPlan {
  /** The deploy the change opened (w3/105); null when it opened none. */
  deployId: string | null;
}

export interface UseUpdatePlanResult {
  /** Fires updateServicePlan; resolves the saved change, or null on failure
   *  (toasted either way). */
  updatePlan: (
    id: string,
    plan: string,
    displayName: string,
  ) => Promise<SavedPlan | null>;
  busy: boolean;
}

/**
 * Wires the plan picker's confirm action to bex-api's `updateServicePlan`
 * (w1/m8). The mutation itself is synchronous (it patches spec.tier and
 * returns immediately); the pod resize/rollout happens after, same as
 * suspend/restart — the toast says so rather than implying an instant apply.
 */
export function useUpdatePlan(): UseUpdatePlanResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(UpdateServicePlanDocument);
  const [busy, setBusy] = useState(false);
  const paymentGate = usePaymentRequiredGate();

  const updatePlan = useCallback(
    async (id: string, plan: string, displayName: string) => {
      setBusy(true);
      try {
        const result = await paymentGate.run(() =>
          mutate({ variables: { id, plan } }),
        );
        toast.success(t("services.planPickerSuccess", { name: displayName }));
        return {
          deployId: result?.data?.updateServicePlan?.latestDeployId ?? null,
        };
      } catch (error) {
        if (isPaymentOnboardingCancelled(error)) return null;
        toast.error(mutationErrorMessage(error, t("services.planPickerError")));
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, paymentGate, t],
  );

  return { updatePlan, busy };
}
