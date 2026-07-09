import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { RevokeApiKeyDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseRevokeApiKeyResult {
  /** Fires revokeApiKey; resolves true on success (toasted either way). */
  revoke: (id: string, name: string) => Promise<boolean>;
  /** The id currently being revoked, or null (disables that row's control). */
  revoking: string | null;
}

/**
 * Wires the revoke action to bex-api's `revokeApiKey`. On failure the caller
 * doesn't remove the row (no optimistic update), so a failed revoke leaves the
 * key listed — the correct behavior since the token is still live.
 */
export function useRevokeApiKey(): UseRevokeApiKeyResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(RevokeApiKeyDocument);
  const [revoking, setRevoking] = useState<string | null>(null);

  const revoke = useCallback(
    async (id: string, name: string) => {
      setRevoking(id);
      try {
        await mutate({ variables: { id } });
        toast.success(t("apiKeys.revokeSuccess", { name }));
        return true;
      } catch {
        toast.error(t("apiKeys.revokeError", { name }));
        return false;
      } finally {
        setRevoking(null);
      }
    },
    [mutate, t],
  );

  return { revoke, revoking };
}
