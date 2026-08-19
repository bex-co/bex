import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateDatabaseDiskAutoscalingDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseUpdateDatabaseDiskAutoscalingResult {
  updateDiskAutoscaling: (id: string, enabled: boolean) => Promise<boolean>;
  busy: boolean;
}

export function useUpdateDatabaseDiskAutoscaling(): UseUpdateDatabaseDiskAutoscalingResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(UpdateDatabaseDiskAutoscalingDocument);
  const [busy, setBusy] = useState(false);

  const updateDiskAutoscaling = useCallback(
    async (id: string, enabled: boolean) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, enabled } });
        toast.success(
          t(
            enabled
              ? "databases.diskAutoscalingEnabled"
              : "databases.diskAutoscalingDisabled",
          ),
        );
        return true;
      } catch {
        toast.error(t("databases.diskAutoscalingError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { updateDiskAutoscaling, busy };
}
