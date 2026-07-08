import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateServicePlanDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseUpdatePlanResult {
  /** Fires updateServicePlan; resolves true on success (toasted either way). */
  updatePlan: (id: string, plan: string, displayName: string) => Promise<boolean>;
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

  const updatePlan = useCallback(
    async (id: string, plan: string, displayName: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, plan } });
        toast.success(t("services.planPickerSuccess", { name: displayName }));
        return true;
      } catch {
        toast.error(t("services.planPickerError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { updatePlan, busy };
}
