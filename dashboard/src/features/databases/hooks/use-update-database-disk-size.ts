import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateDatabaseDiskSizeDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

export interface UseUpdateDatabaseDiskSizeResult {
  /**
   * Grows the database's storage to `diskSizeGB`. Resolves null when accepted,
   * else the message to show: the server's own refusal (grow-only, plan
   * maximum, free plan) when it gave one.
   */
  updateDiskSize: (id: string, diskSizeGB: number) => Promise<string | null>;
  busy: boolean;
}

/** Render's Plan > Storage edit (w2/m307): REST's PATCH diskSizeGB. */
export function useUpdateDatabaseDiskSize(): UseUpdateDatabaseDiskSizeResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(UpdateDatabaseDiskSizeDocument);
  const [busy, setBusy] = useState(false);

  const updateDiskSize = useCallback(
    async (id: string, diskSizeGB: number) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, diskSizeGB } });
        toast.success(t("databases.storageEditSuccess", { size: diskSizeGB }));
        return null;
      } catch (err) {
        return mutationErrorMessage(err, t("databases.storageEditError"));
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { updateDiskSize, busy };
}
