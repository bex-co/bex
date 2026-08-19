import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { RenameDatabaseDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseRenameDatabaseResult {
  rename: (id: string, name: string) => Promise<boolean>;
  busy: boolean;
}

export function useRenameDatabase(): UseRenameDatabaseResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(RenameDatabaseDocument);
  const [busy, setBusy] = useState(false);

  const rename = useCallback(
    async (id: string, name: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, name } });
        toast.success(t("databases.nameSuccess", { name }));
        return true;
      } catch (error) {
        const message = error instanceof Error ? error.message : "";
        if (message.includes("already exists")) {
          toast.error(t("databases.nameConflict"));
        } else if (message.includes("name must")) {
          toast.error(t("databases.nameInvalid"));
        } else {
          toast.error(t("databases.nameError"));
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
