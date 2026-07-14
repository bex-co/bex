import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateDatabasePlanDocument } from "@/features/databases/api/operations";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseUpdateDatabasePlanResult {
  updatePlan: (id: string, plan: string, displayName: string) => Promise<boolean>;
  busy: boolean;
}

export function useUpdateDatabasePlan(): UseUpdateDatabasePlanResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(UpdateDatabasePlanDocument);
  const [busy, setBusy] = useState(false);

  const updatePlan = useCallback(
    async (id: string, plan: string, displayName: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, plan } });
        toast.success(t("databases.planPickerSuccess", { name: displayName }));
        return true;
      } catch {
        toast.error(t("databases.planPickerError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { updatePlan, busy };
}
