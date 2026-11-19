import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CreateApiKeyDocument,
  type ApiKeyPermissionsInput,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { bearerKey } from "@/features/api-keys/lib/bearer-key";
import type { CreatedApiKey } from "@/features/api-keys/types";

export interface UseCreateApiKeyResult {
  /**
   * Fires createApiKey; resolves the minted key (its bearer string and secret
   * included) or null. `permissions` is the key's grant (w9/m496); omitted,
   * bex-api grants All, as Render does.
   */
  create: (
    name: string,
    permissions?: ApiKeyPermissionsInput,
  ) => Promise<CreatedApiKey | null>;
  busy: boolean;
}

/**
 * Wires the mint dialog to bex-api's `createApiKey`. `fetchPolicy: "no-cache"`
 * is deliberate: the key and its secret are returned exactly once by design
 * (docs/ADR012-auth.md), and Apollo's normalized cache would otherwise key the
 * response under `ApiKey:<id>` and hold them there indefinitely — reachable by
 * any later cache read even after the mint dialog dismisses it. no-cache means
 * the response only ever exists in this hook's return value and the dialog's
 * own state, both cleared on dismiss (t003's "unretrievable after dismiss").
 */
export function useCreateApiKey(): UseCreateApiKeyResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(CreateApiKeyDocument, {
    fetchPolicy: "no-cache",
  });
  const [busy, setBusy] = useState(false);

  const create = useCallback(
    async (name: string, permissions?: ApiKeyPermissionsInput) => {
      // Scoped to the switcher's selected workspace (w6/m18) — refused (never
      // sent with a null ownerId, which the backend would silently route to the
      // caller's default workspace) until the workspace list resolves, mirroring
      // useCreateService.
      if (currentWorkspaceId == null) {
        toast.error(t("apiKeys.createError", { name }));
        return null;
      }
      setBusy(true);
      try {
        const res = await mutate({
          variables: {
            name,
            ownerId: currentWorkspaceId,
            ...(permissions ? { permissions } : {}),
          },
        });
        const minted = res.data?.createApiKey;
        if (!minted?.id || !minted.secret) {
          throw new Error("createApiKey returned no secret");
        }
        toast.success(t("apiKeys.createSuccess", { name }));
        return {
          id: minted.id,
          name: minted.name ?? name,
          key: bearerKey(minted.id, minted.secret, minted.key),
          secret: minted.secret,
        };
      } catch (err) {
        toast.error(
          mutationErrorMessage(err, t("apiKeys.createError", { name })),
        );
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { create, busy };
}
