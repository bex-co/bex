import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DeleteDatabaseDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseDeleteDatabaseResult {
  /** Fires deleteDatabase; resolves true on success (toasted either way). */
  remove: (id: string, name: string) => Promise<boolean>;
  /** The id currently being deleted, or null (disables that row's control). */
  deleting: string | null;
}

/**
 * Wires the delete action to bex-api's `deleteDatabase`, which cascades the CNPG
 * Cluster + PVC + Secret + external route (docs/ADR006-bex-api.md §Managed Postgres).
 * Destructive and irreversible, so callers gate it behind a typed confirm.
 */
export function useDeleteDatabase(): UseDeleteDatabaseResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(DeleteDatabaseDocument);
  const [deleting, setDeleting] = useState<string | null>(null);

  const remove = useCallback(
    async (id: string, name: string) => {
      setDeleting(id);
      try {
        await mutate({ variables: { id } });
        toast.success(t("databases.deleteSuccess", { name }));
        return true;
      } catch {
        toast.error(t("databases.deleteError", { name }));
        return false;
      } finally {
        setDeleting(null);
      }
    },
    [mutate, t],
  );

  return { remove, deleting };
}
