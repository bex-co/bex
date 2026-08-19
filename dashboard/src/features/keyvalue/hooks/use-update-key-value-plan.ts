import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateKeyValuePlanDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";

export interface UseUpdateKeyValuePlanResult {
  updatePlan: (
    id: string,
    plan: string,
    displayName: string,
  ) => Promise<boolean>;
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
        return true;
      } catch (error) {
        if (isPaymentOnboardingCancelled(error)) return false;
        toast.error(t("keyvalue.planPickerError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, paymentGate, t],
  );

  return { updatePlan, busy };
}
