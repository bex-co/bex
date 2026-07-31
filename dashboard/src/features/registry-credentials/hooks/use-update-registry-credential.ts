import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateRegistryCredentialDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UpdateRegistryCredentialInput {
  id: string;
  /** New display name; omit/blank leaves it unchanged. */
  name?: string;
  /** New registry username; omit/blank leaves it unchanged. */
  username?: string;
  /**
   * A rotated token. The secret is write-only — a blank value keeps the stored
   * token (never echoed back), so an edit that only renames sends no token.
   */
  authToken?: string;
  /** RFC3339 expiry; omit/blank leaves it unchanged. */
  expiresAt?: string;
}

export interface UseUpdateRegistryCredentialResult {
  /** Fires updateRegistryCredential; resolves true on success (toasted either way). */
  update: (input: UpdateRegistryCredentialInput) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the edit dialog to bex-api's `updateRegistryCredential` (w5/m60): rename,
 * change the username, rotate the token, or update the expiry. A blank token
 * sends `null`, which the server reads as "keep the existing secret" — the token
 * is never rendered, so this never round-trips a stored value.
 */
export function useUpdateRegistryCredential(): UseUpdateRegistryCredentialResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(UpdateRegistryCredentialDocument);
  const [busy, setBusy] = useState(false);

  const update = useCallback(
    async (input: UpdateRegistryCredentialInput) => {
      setBusy(true);
      try {
        await mutate({
          variables: {
            id: input.id,
            name: input.name?.trim() || null,
            username: input.username?.trim() || null,
            // Blank => omit the secret so the stored token is preserved.
            authToken: input.authToken?.trim() || null,
            expiresAt: input.expiresAt?.trim() || null,
          },
        });
        toast.success(t("registryCredentials.updateSuccess"));
        return true;
      } catch {
        toast.error(t("registryCredentials.updateError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { update, busy };
}
