import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DeleteEnvironmentDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseDeleteEnvironmentResult {
  /** Fires deleteEnvironment; resolves true on success (toasted either way). */
  remove: (id: string, name: string) => Promise<boolean>;
  /** The id currently being deleted, or null (disables that environment's control). */
  deleting: string | null;
}

/**
 * Wires an environment's Delete action to bex-api's `deleteEnvironment` — its
 * services keep their project membership, they just lose the environment label
 * (docs/ADR032-environments.md).
 */
export function useDeleteEnvironment(): UseDeleteEnvironmentResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(DeleteEnvironmentDocument, {
    refetchQueries: ["Environments"],
    awaitRefetchQueries: true,
  });
  const [deleting, setDeleting] = useState<string | null>(null);

  const remove = useCallback(
    async (id: string, name: string) => {
      setDeleting(id);
      try {
        await mutate({ variables: { id } });
        toast.success(t("environments.deleteSuccess", { name }));
        return true;
      } catch {
        toast.error(t("environments.deleteError", { name }));
        return false;
      } finally {
        setDeleting(null);
      }
    },
    [mutate, t],
  );

  return { remove, deleting };
}
