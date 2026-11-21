import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useTranslations } from "@/common/hooks/use-translations";
import { UpdateDatabaseVersionDocument } from "@/graphql/definitions";

export function useUpdateDatabaseVersion() {
  const { t } = useTranslations();
  const [mutate] = useMutation(UpdateDatabaseVersionDocument);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const updateVersion = useCallback(
    async (id: string, version: string) => {
      setBusy(true);
      setError(null);
      try {
        const res = await mutate({ variables: { id, version } });
        // The server says what the accepted upgrade drops (a publisher's
        // logical replication slots before PostgreSQL 17, w34/m543): then the
        // toast warns, with its words, and stays until it is dismissed.
        const accepted = t("databases.versionUpgradeAccepted", { version });
        const notice = res.data?.updateDatabaseVersion?.upgradeNotice;
        if (notice) {
          toast.warning(accepted, { description: notice, duration: Infinity });
        } else {
          toast.success(accepted);
        }
        return true;
      } catch (err) {
        setError(mutationErrorMessage(err, t("databases.versionUpgradeError")));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return {
    updateVersion,
    busy,
    error,
    clearError: () => setError(null),
  };
}
