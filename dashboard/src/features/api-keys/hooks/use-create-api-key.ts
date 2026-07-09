import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateApiKeyDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import type { CreatedApiKey } from "@/features/api-keys/types";

export interface UseCreateApiKeyResult {
  /** Fires createApiKey; resolves the minted key (secret included) or null. */
  create: (name: string) => Promise<CreatedApiKey | null>;
  busy: boolean;
}

/**
 * Wires the mint dialog to bex-api's `createApiKey`. `fetchPolicy: "no-cache"`
 * is deliberate: the secret is returned exactly once by design (docs/auth.md),
 * and Apollo's normalized cache would otherwise key the response under
 * `ApiKey:<id>` and hold the secret there indefinitely — reachable by any later
 * cache read even after the mint dialog dismisses it. no-cache means the
 * response only ever exists in this hook's return value and the dialog's own
 * state, both cleared on dismiss (t003's "unretrievable after dismiss").
 */
export function useCreateApiKey(): UseCreateApiKeyResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(CreateApiKeyDocument, { fetchPolicy: "no-cache" });
  const [busy, setBusy] = useState(false);

  const create = useCallback(
    async (name: string) => {
      setBusy(true);
      try {
        const res = await mutate({ variables: { name } });
        const key = res.data?.createApiKey;
        if (!key?.id || !key.secret) {
          throw new Error("createApiKey returned no secret");
        }
        toast.success(t("apiKeys.createSuccess", { name }));
        return { id: key.id, name: key.name ?? name, secret: key.secret };
      } catch {
        toast.error(t("apiKeys.createError", { name }));
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { create, busy };
}
