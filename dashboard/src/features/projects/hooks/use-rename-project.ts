import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { RenameProjectDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseRenameProjectResult {
  /** Fires renameProject; resolves true on success (toasted either way). */
  rename: (id: string, name: string) => Promise<boolean>;
  busy: boolean;
}

/** Wires the project "•••" menu's Rename action to bex-api's `renameProject`. */
export function useRenameProject(): UseRenameProjectResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(RenameProjectDocument);
  const [busy, setBusy] = useState(false);

  const rename = useCallback(
    async (id: string, name: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, name } });
        toast.success(t("projects.renameSuccess", { name }));
        return true;
      } catch {
        toast.error(t("projects.renameError", { name }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { rename, busy };
}
