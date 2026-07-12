import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DeleteServiceDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseDeleteServiceResult {
  /** Fires deleteService; resolves true on success (toasted either way). */
  remove: (id: string, name: string) => Promise<boolean>;
  /** True while the delete is in flight (disables the confirm control). */
  deleting: boolean;
}

/**
 * Wires the danger-zone delete to bex-api's `deleteService`, which tears down
 * the App's Deployment/Service/Ingress (the delete half of Create/Suspend/Delete,
 * w2/m4). Destructive and irreversible, so callers gate it behind a typed-name
 * confirm. On success the deleted `Service` is evicted from the normalized cache
 * so the services list drops the row immediately — no stale row flashes before
 * the list's own refetch lands.
 */
export function useDeleteService(): UseDeleteServiceResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(DeleteServiceDocument);
  const [deleting, setDeleting] = useState(false);

  const remove = useCallback(
    async (id: string, name: string) => {
      setDeleting(true);
      try {
        await mutate({
          variables: { id },
          update(cache) {
            cache.evict({ id: cache.identify({ __typename: "Service", id }) });
            cache.gc();
          },
        });
        toast.success(t("services.deleteSuccess", { name }));
        return true;
      } catch {
        toast.error(t("services.deleteError", { name }));
        return false;
      } finally {
        setDeleting(false);
      }
    },
    [mutate, t],
  );

  return { remove, deleting };
}
