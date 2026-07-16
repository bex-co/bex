import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { RenameKeyValueDocument } from "@/features/keyvalue/api/operations";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseRenameKeyValueResult {
  rename: (id: string, name: string) => Promise<boolean>;
  busy: boolean;
}

export function useRenameKeyValue(): UseRenameKeyValueResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(RenameKeyValueDocument);
  const [busy, setBusy] = useState(false);

  const rename = useCallback(
    async (id: string, name: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, name } });
        toast.success(t("keyvalue.nameSuccess", { name }));
        return true;
      } catch (error) {
        const message = error instanceof Error ? error.message : "";
        if (message.includes("already exists")) {
          toast.error(t("keyvalue.nameConflict"));
        } else if (message.includes("name must")) {
          toast.error(t("keyvalue.nameInvalid"));
        } else {
          toast.error(t("keyvalue.nameError"));
        }
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { rename, busy };
}
