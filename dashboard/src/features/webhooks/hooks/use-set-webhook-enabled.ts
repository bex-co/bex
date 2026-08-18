import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetWebhookEndpointEnabledDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { webhookErrorMessageKey } from "@/features/webhooks/lib/errors";

export interface UseSetWebhookEnabledResult {
  /** Flips an endpoint's enabled flag; resolves true on success. */
  setEnabled: (id: string, name: string, enabled: boolean) => Promise<boolean>;
  /** The id currently being toggled, or null (disables that row's switch). */
  toggling: string | null;
}

/**
 * Wires the per-endpoint enable/disable switch to bex-api's
 * `setWebhookEndpointEnabled` — also how an auto-disabled endpoint (repeated
 * delivery failures) is re-armed after its destination is fixed.
 */
export function useSetWebhookEnabled(): UseSetWebhookEnabledResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(SetWebhookEndpointEnabledDocument);
  const [toggling, setToggling] = useState<string | null>(null);

  const setEnabled = useCallback(
    async (id: string, name: string, enabled: boolean) => {
      setToggling(id);
      try {
        await mutate({
          variables: { id, enabled, ownerId: currentWorkspaceId },
        });
        toast.success(
          t(enabled ? "webhooks.enableSuccess" : "webhooks.disableSuccess", {
            name,
          }),
        );
        return true;
      } catch (error) {
        const key = webhookErrorMessageKey(error);
        toast.error(key ? t(key) : t("webhooks.toggleError", { name }));
        return false;
      } finally {
        setToggling(null);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { setEnabled, toggling };
}
