import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DeleteRegistryCredentialDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseDeleteRegistryCredentialResult {
  /** Fires deleteRegistryCredential; resolves true on success (toasted either way). */
  remove: (id: string, name: string) => Promise<boolean>;
  /** The id currently being deleted, or null (disables that row's control). */
  deleting: string | null;
}

/**
 * Wires the delete action to bex-api's `deleteRegistryCredential`. On failure
 * the caller doesn't remove the row (no optimistic update) — a failed delete
 * leaves the credential listed, the correct behavior since it's still stored.
 */
export function useDeleteRegistryCredential(): UseDeleteRegistryCredentialResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(DeleteRegistryCredentialDocument);
  const [deleting, setDeleting] = useState<string | null>(null);

  const remove = useCallback(
    async (id: string, name: string) => {
      setDeleting(id);
      try {
        await mutate({ variables: { id } });
        toast.success(t("registryCredentials.deleteSuccess", { name }));
        return true;
      } catch {
        toast.error(t("registryCredentials.deleteError", { name }));
        return false;
      } finally {
        setDeleting(null);
      }
    },
    [mutate, t],
  );

  return { remove, deleting };
}
