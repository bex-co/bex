import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateRegistryCredentialDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface CreateRegistryCredentialInput {
  host: string;
  username: string;
  authToken: string;
  /** Display label; blank defaults to host server-side. */
  name?: string;
  /** RFC3339; blank means the credential never expires. */
  expiresAt?: string;
}

export interface UseCreateRegistryCredentialResult {
  /** Fires createRegistryCredential; resolves true on success (toasted either way). */
  create: (input: CreateRegistryCredentialInput) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the create form to bex-api's `createRegistryCredential`. Unlike the
 * API-key mint flow, the secret here is caller-supplied (not server-
 * generated) — there's nothing to reveal back, so no "shown once" dialog
 * step; a plain success toast is enough. The server's response has no secret
 * field to leak into Apollo's cache in the first place.
 */
export function useCreateRegistryCredential(): UseCreateRegistryCredentialResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(CreateRegistryCredentialDocument);
  const [busy, setBusy] = useState(false);

  const create = useCallback(
    async (input: CreateRegistryCredentialInput) => {
      setBusy(true);
      try {
        await mutate({
          variables: {
            host: input.host,
            username: input.username,
            authToken: input.authToken,
            name: input.name || null,
            expiresAt: input.expiresAt || null,
          },
        });
        toast.success(
          t("registryCredentials.createSuccess", { host: input.host }),
        );
        return true;
      } catch {
        toast.error(t("registryCredentials.createError", { host: input.host }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { create, busy };
}
