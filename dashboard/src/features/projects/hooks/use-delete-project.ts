import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DeleteProjectDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseDeleteProjectResult {
  /** Fires deleteProject; resolves true on success (toasted either way). */
  remove: (id: string, name: string) => Promise<boolean>;
  /** The id currently being deleted, or null (disables that project's control). */
  deleting: string | null;
}

/**
 * Wires the project "•••" menu's Delete action to bex-api's `deleteProject` —
 * its services/databases/key-values become unassigned, never deleted.
 */
export function useDeleteProject(): UseDeleteProjectResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(DeleteProjectDocument);
  const [deleting, setDeleting] = useState<string | null>(null);

  const remove = useCallback(
    async (id: string, name: string) => {
      setDeleting(id);
      try {
        await mutate({ variables: { id } });
        toast.success(t("projects.deleteSuccess", { name }));
        return true;
      } catch {
        toast.error(t("projects.deleteError", { name }));
        return false;
      } finally {
        setDeleting(null);
      }
    },
    [mutate, t],
  );

  return { remove, deleting };
}
