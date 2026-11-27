import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateKeyValuePlanDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";

export interface UseUpdateKeyValuePlanResult {
  /**
   * Applies the plan. `ok` when accepted (a success toast says so); otherwise
   * `error` is the message the plan review shows in place, absent when the
   * user closed the payment-method step themselves (w3/m674).
   */
  updatePlan: (
    id: string,
    plan: string,
    displayName: string,
  ) => Promise<{ ok: boolean; error?: string }>;
  busy: boolean;
}

export function useUpdateKeyValuePlan(): UseUpdateKeyValuePlanResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(UpdateKeyValuePlanDocument);
  const [busy, setBusy] = useState(false);
  const paymentGate = usePaymentRequiredGate();

  const updatePlan = useCallback(
    async (id: string, plan: string, displayName: string) => {
      setBusy(true);
      try {
        await paymentGate.run(() => mutate({ variables: { id, plan } }));
        toast.success(t("keyvalue.planPickerSuccess", { name: displayName }));
        return { ok: true };
      } catch (error) {
        if (isPaymentOnboardingCancelled(error)) return { ok: false };
        return {
          ok: false,
          error: mutationErrorMessage(error, t("keyvalue.planPickerError")),
        };
      } finally {
        setBusy(false);
      }
    },
    [mutate, paymentGate, t],
  );

  return { updatePlan, busy };
}
