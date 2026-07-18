import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DeleteDatabaseDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  protectedConfirmationFromError,
  type ProtectedActionResult,
} from "@/features/services/lib/protected-confirmation";

export interface UseDeleteDatabaseResult {
  /** Fires deleteDatabase; resolves true on success (toasted either way). */
  remove: (
    id: string,
    name: string,
    confirmation?: string,
  ) => Promise<ProtectedActionResult>;
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
    async (id: string, name: string, confirmation?: string) => {
      setDeleting(id);
      try {
        await mutate({ variables: { id, confirm: confirmation } });
        toast.success(t("databases.deleteSuccess", { name }));
        return { status: "success" } as const;
      } catch (err) {
        const requiredConfirmation = protectedConfirmationFromError(err);
        if (requiredConfirmation) {
          return {
            status: "confirmation_required",
            confirmation: requiredConfirmation,
          } as const;
        }
        toast.error(t("databases.deleteError", { name }));
        return { status: "error" } as const;
      } finally {
        setDeleting(null);
      }
    },
    [mutate, t],
  );

  return { remove, deleting };
}
