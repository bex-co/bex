import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { RenameEnvironmentDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

export interface UseRenameEnvironmentResult {
  /** Fires renameEnvironment; resolves true on success (toasted either way). */
  rename: (id: string, name: string) => Promise<boolean>;
  busy: boolean;
}

/** Wires an environment's Rename action to bex-api's `renameEnvironment`. */
export function useRenameEnvironment(): UseRenameEnvironmentResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(RenameEnvironmentDocument, {
    refetchQueries: ["Environments"],
    awaitRefetchQueries: true,
  });
  const [busy, setBusy] = useState(false);

  const rename = useCallback(
    async (id: string, name: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, name } });
        toast.success(t("environments.renameSuccess", { name }));
        return true;
      } catch (err) {
        toast.error(
          mutationErrorMessage(err, t("environments.renameError", { name })),
        );
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { rename, busy };
}
