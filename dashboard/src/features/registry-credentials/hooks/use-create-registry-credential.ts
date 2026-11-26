import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateRegistryCredentialDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

export interface CreateRegistryCredentialInput {
  /** Render's registry enum (GITHUB, GITLAB, DOCKER, GOOGLE_ARTIFACT, AWS_ECR). */
  registry?: string;
  /**
   * The registry host: needed with Google Artifact and ECR, whose hosts name
   * a project or account and region, and alone for a self-hosted registry.
   */
  host?: string;
  username: string;
  authToken: string;
  /** Display label; blank defaults to host server-side. */
  name?: string;
  /** RFC3339; blank means the credential never expires. */
  expiresAt?: string;
}

export interface UseCreateRegistryCredentialResult {
  /**
   * Fires createRegistryCredential; resolves the new credential's id on
   * success and null on failure (toasted either way).
   */
  create: (input: CreateRegistryCredentialInput) => Promise<string | null>;
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
  const { currentWorkspaceId } = useWorkspace();
  const { t } = useTranslations();
  const [mutate] = useMutation(CreateRegistryCredentialDocument);
  const [busy, setBusy] = useState(false);

  const create = useCallback(
    async (input: CreateRegistryCredentialInput) => {
      setBusy(true);
      try {
        const { data } = await mutate({
          variables: {
            ownerId: currentWorkspaceId,
            registry: input.registry || null,
            host: input.host || null,
            username: input.username,
            authToken: input.authToken,
            name: input.name || null,
            expiresAt: input.expiresAt || null,
          },
        });
        const created = data?.createRegistryCredential;
        if (!created?.id) {
          throw new Error("createRegistryCredential returned no credential");
        }
        // Name the host bex-api stored (it drops a pasted scheme or trailing
        // slash, folds Docker Hub aliases and fills in a registry's one host),
        // not the text typed (w4/149).
        const host = created.host || input.host || "";
        toast.success(t("registryCredentials.createSuccess", { host }));
        return created.id;
      } catch (err) {
        toast.error(
          mutationErrorMessage(err, t("registryCredentials.createError")),
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
