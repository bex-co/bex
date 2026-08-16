import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateWebhookEndpointDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface WebhookUpdate {
  name?: string;
  url?: string;
  eventTypes?: string[];
  enabled?: boolean;
}

export interface UseUpdateWebhookResult {
  /** Fires updateWebhookEndpoint; resolves true on success (toasted either way). */
  update: (id: string, patch: WebhookUpdate) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings tab to bex-api's `updateWebhookEndpoint` (w1/m49/t005 —
 * the verb itself shipped with w3/m11 and was never wired to the dashboard).
 * Omitted patch fields keep their current values server-side (service.Update's
 * merge), matching REST `PATCH /v1/webhooks/{id}` semantics. The mutation's
 * selection set updates the normalized cache row in place, so the list and
 * detail views refresh without a refetch.
 */
export function useUpdateWebhook(): UseUpdateWebhookResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(UpdateWebhookEndpointDocument);
  const [busy, setBusy] = useState(false);

  const update = useCallback(
    async (id: string, patch: WebhookUpdate) => {
      setBusy(true);
      try {
        await mutate({
          variables: { id, ownerId: currentWorkspaceId, ...patch },
        });
        toast.success(t("webhooks.updateSuccess"));
        return true;
      } catch (err) {
        // Surface the backend's named refusal (invalid URL, empty event set)
        // instead of a generic failure — parity with how Create toasts.
        const detail = err instanceof Error && err.message ? err.message : "";
        toast.error(
          detail
            ? `${t("webhooks.updateError")}: ${detail}`
            : t("webhooks.updateError"),
        );
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { update, busy };
}
