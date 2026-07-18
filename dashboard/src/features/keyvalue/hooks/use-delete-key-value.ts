import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DeleteKeyValueDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  protectedConfirmationFromError,
  type ProtectedActionResult,
} from "@/features/services/lib/protected-confirmation";

export interface UseDeleteKeyValueResult {
  /** Fires deleteKeyValue; resolves true on success (toasted either way). */
  remove: (
    id: string,
    name: string,
    confirmation?: string,
  ) => Promise<ProtectedActionResult>;
  /** The id currently being deleted, or null (disables that row's control). */
  deleting: string | null;
}

/**
 * Wires the delete action to bex-api's `deleteKeyValue`, which cascades the
 * Valkey StatefulSet + PVC + Secret + external route
 * (docs/ADR021-keyvalue-management.md). Destructive and irreversible, so callers gate
 * it behind a typed confirm. Mirrors databases' `useDeleteDatabase`.
 */
export function useDeleteKeyValue(): UseDeleteKeyValueResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(DeleteKeyValueDocument);
  const [deleting, setDeleting] = useState<string | null>(null);

  const remove = useCallback(
    async (id: string, name: string, confirmation?: string) => {
      setDeleting(id);
      try {
        await mutate({ variables: { id, confirm: confirmation } });
        toast.success(t("keyvalue.deleteSuccess", { name }));
        return { status: "success" } as const;
      } catch (err) {
        const requiredConfirmation = protectedConfirmationFromError(err);
        if (requiredConfirmation) {
          return {
            status: "confirmation_required",
            confirmation: requiredConfirmation,
          } as const;
        }
        toast.error(t("keyvalue.deleteError", { name }));
        return { status: "error" } as const;
      } finally {
        setDeleting(null);
      }
    },
    [mutate, t],
  );

  return { remove, deleting };
}
