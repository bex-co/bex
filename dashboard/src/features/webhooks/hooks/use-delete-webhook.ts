import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DeleteWebhookEndpointDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { webhookErrorMessageKey } from "@/features/webhooks/lib/errors";

export interface UseDeleteWebhookResult {
  /** Fires deleteWebhookEndpoint; resolves true on success (toasted either way). */
  remove: (id: string, name: string) => Promise<boolean>;
  /** The id currently being deleted, or null (disables that row's control). */
  deleting: string | null;
}

/**
 * Wires the delete action to bex-api's `deleteWebhookEndpoint`. No optimistic
 * removal: a failed delete leaves the endpoint listed, which is correct since
 * it would still receive events.
 */
export function useDeleteWebhook(): UseDeleteWebhookResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(DeleteWebhookEndpointDocument);
  const [deleting, setDeleting] = useState<string | null>(null);

  const remove = useCallback(
    async (id: string, name: string) => {
      setDeleting(id);
      try {
        await mutate({ variables: { id, ownerId: currentWorkspaceId } });
        toast.success(t("webhooks.deleteSuccess", { name }));
        return true;
      } catch (error) {
        const key = webhookErrorMessageKey(error);
        toast.error(key ? t(key) : t("webhooks.deleteError", { name }));
        return false;
      } finally {
        setDeleting(null);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { remove, deleting };
}
