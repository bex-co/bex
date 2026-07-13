import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { RenameWorkspaceDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { graphQLErrorMessage } from "@/common/lib/graphql-error";

export interface UseRenameWorkspaceResult {
  /** Fires renameWorkspace; resolves true on success. */
  rename: (id: string, name: string) => Promise<boolean>;
  busy: boolean;
  /** The backend's rejection message, shown inline next to the rename form. */
  error: string | null;
}

/**
 * Wires the workspace settings rename form to bex-api's `renameWorkspace`
 * (w6/m1, admin-only). The id stays the key, so a rename breaks no references
 * (switcher/URLs/App CR names) — only the display name changes.
 */
export function useRenameWorkspace(): UseRenameWorkspaceResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(RenameWorkspaceDocument);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const rename = useCallback(
    async (id: string, name: string) => {
      setBusy(true);
      setError(null);
      try {
        await mutate({ variables: { id, name } });
        toast.success(t("workspaces.renameSuccess", { name }));
        return true;
      } catch (err) {
        setError(graphQLErrorMessage(err) ?? t("workspaces.renameError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { rename, busy, error };
}
