import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateWebhookEndpointDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  WebhookMutationError,
  toWebhookMutationError,
} from "@/features/webhooks/lib/errors";

export interface WebhookUpdate {
  name?: string;
  url?: string;
  eventTypes?: string[];
  enabled?: boolean;
}

/** The Name/URL the server accepted — it trims both (service.Update). */
export interface AcceptedWebhookFields {
  name: string;
  url: string;
}

export interface UseUpdateWebhookResult {
  /**
   * Fires updateWebhookEndpoint (toasted either way). Resolves the accepted
   * Name/URL on success; null when the save failed or the response carried
   * no accepted record, so callers keep their drafts.
   */
  update: (
    id: string,
    patch: WebhookUpdate,
  ) => Promise<AcceptedWebhookFields | null>;
  busy: boolean;
  error: Error | null;
  clearError: () => void;
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
  const [error, setError] = useState<Error | null>(null);

  const update = useCallback(
    async (id: string, patch: WebhookUpdate) => {
      setError(null);
      setBusy(true);
      try {
        const { data } = await mutate({
          variables: { id, ownerId: currentWorkspaceId, ...patch },
        });
        toast.success(t("webhooks.updateSuccess"));
        const accepted = data?.updateWebhookEndpoint;
        return accepted?.name != null && accepted.url != null
          ? { name: accepted.name, url: accepted.url }
          : null;
      } catch (err) {
        const normalized = toWebhookMutationError(err);
        if (normalized instanceof WebhookMutationError) {
          setError(normalized);
        } else {
          const message = mutationErrorMessage(err, t("webhooks.updateError"));
          setError(new Error(message));
          toast.error(message);
        }
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  const clearError = useCallback(() => setError(null), []);
  return { update, busy, error, clearError };
}
