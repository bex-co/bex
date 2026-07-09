import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DeleteKeyValueDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseDeleteKeyValueResult {
  /** Fires deleteKeyValue; resolves true on success (toasted either way). */
  remove: (id: string, name: string) => Promise<boolean>;
  /** The id currently being deleted, or null (disables that row's control). */
  deleting: string | null;
}

/**
 * Wires the delete action to bex-api's `deleteKeyValue`, which cascades the
 * Valkey StatefulSet + PVC + Secret + external route
 * (docs/keyvalue-management.md). Destructive and irreversible, so callers gate
 * it behind a typed confirm. Mirrors databases' `useDeleteDatabase`.
 */
export function useDeleteKeyValue(): UseDeleteKeyValueResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(DeleteKeyValueDocument);
  const [deleting, setDeleting] = useState<string | null>(null);

  const remove = useCallback(
    async (id: string, name: string) => {
      setDeleting(id);
      try {
        await mutate({ variables: { id } });
        toast.success(t("keyvalue.deleteSuccess", { name }));
        return true;
      } catch {
        toast.error(t("keyvalue.deleteError", { name }));
        return false;
      } finally {
        setDeleting(null);
      }
    },
    [mutate, t],
  );

  return { remove, deleting };
}
