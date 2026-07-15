import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { graphQLErrorMessage } from "@/common/lib/graphql-error";
import { useTranslations } from "@/common/hooks/use-translations";
import { UpdateDatabaseVersionDocument } from "@/features/databases/api/operations";

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
        await mutate({ variables: { id, version } });
        toast.success(t("databases.versionUpgradeAccepted", { version }));
        return true;
      } catch (err) {
        setError(
          graphQLErrorMessage(err) ?? t("databases.versionUpgradeError"),
        );
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
