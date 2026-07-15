import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DeleteServiceDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  protectedConfirmationFromError,
  type ProtectedActionResult,
} from "@/features/services/lib/protected-confirmation";

export interface UseDeleteServiceResult {
  /** Fires deleteService and surfaces an authoritative protected retry phrase. */
  remove: (
    id: string,
    name: string,
    confirmation?: string,
  ) => Promise<ProtectedActionResult>;
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
    async (id: string, name: string, confirmation?: string) => {
      setDeleting(true);
      try {
        await mutate({
          variables: { id, confirm: confirmation },
          update(cache) {
            cache.evict({ id: cache.identify({ __typename: "Service", id }) });
            cache.gc();
          },
        });
        toast.success(t("services.deleteSuccess", { name }));
        return { status: "success" } as const;
      } catch (err) {
        const requiredConfirmation = protectedConfirmationFromError(err);
        if (requiredConfirmation) {
          return {
            status: "confirmation_required",
            confirmation: requiredConfirmation,
          } as const;
        }
        toast.error(t("services.deleteError", { name }));
        return { status: "error" } as const;
      } finally {
        setDeleting(false);
      }
    },
    [mutate, t],
  );

  return { remove, deleting };
}
