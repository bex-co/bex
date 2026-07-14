import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateKeyValuePlanDocument } from "@/features/keyvalue/api/operations";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseUpdateKeyValuePlanResult {
  updatePlan: (id: string, plan: string, displayName: string) => Promise<boolean>;
  busy: boolean;
}

export function useUpdateKeyValuePlan(): UseUpdateKeyValuePlanResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(UpdateKeyValuePlanDocument);
  const [busy, setBusy] = useState(false);

  const updatePlan = useCallback(
    async (id: string, plan: string, displayName: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, plan } });
        toast.success(t("keyvalue.planPickerSuccess", { name: displayName }));
        return true;
      } catch {
        toast.error(t("keyvalue.planPickerError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { updatePlan, busy };
}
