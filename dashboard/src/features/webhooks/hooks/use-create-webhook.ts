import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateWebhookEndpointDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { CreatedWebhookEndpoint } from "@/features/webhooks/types";

export interface UseCreateWebhookResult {
  /** Fires createWebhookEndpoint; resolves the endpoint (secret included) or null. */
  create: (
    name: string,
    url: string,
    eventTypes: string[],
  ) => Promise<CreatedWebhookEndpoint | null>;
  busy: boolean;
}

/**
 * Wires the create dialog to bex-api's `createWebhookEndpoint`.
 * `fetchPolicy: "no-cache"` is deliberate and load-bearing: the signing
 * secret is returned exactly once by design, and Apollo's normalized cache
 * would otherwise hold it under `WebhookEndpoint:<id>` indefinitely —
 * reachable by any later cache read even after the dialog dismisses it.
 * no-cache means the response only ever exists in this hook's return value
 * and the dialog's own state, both cleared on dismiss (the API-key mint
 * pattern, w4/m8).
 */
export function useCreateWebhook(): UseCreateWebhookResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(CreateWebhookEndpointDocument, {
    fetchPolicy: "no-cache",
  });
  const [busy, setBusy] = useState(false);

  const create = useCallback(
    async (name: string, url: string, eventTypes: string[]) => {
      // Refused (never sent with a null ownerId, which the backend would
      // silently route to the caller's default workspace) until the workspace
      // list resolves, mirroring useCreateApiKey.
      if (currentWorkspaceId == null) {
        toast.error(t("webhooks.createError"));
        return null;
      }
      setBusy(true);
      try {
        const res = await mutate({
          variables: { name, url, eventTypes, ownerId: currentWorkspaceId },
        });
        const endpoint = res.data?.createWebhookEndpoint;
        if (!endpoint?.id || !endpoint.secret) {
          throw new Error("createWebhookEndpoint returned no secret");
        }
        toast.success(t("webhooks.createSuccess"));
        return {
          id: endpoint.id,
          name: endpoint.name ?? name,
          secret: endpoint.secret,
        };
      } catch {
        toast.error(t("webhooks.createError"));
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { create, busy };
}
