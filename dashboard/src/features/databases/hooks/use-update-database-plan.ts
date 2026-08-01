import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateDatabasePlanDocument } from "@/features/databases/api/operations";
import { useTranslations } from "@/common/hooks/use-translations";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";

export interface UseUpdateDatabasePlanResult {
  updatePlan: (
    id: string,
    plan: string,
    displayName: string,
  ) => Promise<boolean>;
  busy: boolean;
}

export function useUpdateDatabasePlan(): UseUpdateDatabasePlanResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(UpdateDatabasePlanDocument);
  const [busy, setBusy] = useState(false);
  const paymentGate = usePaymentRequiredGate();

  const updatePlan = useCallback(
    async (id: string, plan: string, displayName: string) => {
      setBusy(true);
      try {
        await paymentGate.run(() => mutate({ variables: { id, plan } }));
        toast.success(t("databases.planPickerSuccess", { name: displayName }));
        return true;
      } catch (error) {
        if (isPaymentOnboardingCancelled(error)) return false;
        toast.error(t("databases.planPickerError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, paymentGate, t],
  );

  return { updatePlan, busy };
}
