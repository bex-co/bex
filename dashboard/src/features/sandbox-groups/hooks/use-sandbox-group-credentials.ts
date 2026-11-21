import { useCallback, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CreateSandboxGroupCredentialDocument,
  DeleteSandboxGroupCredentialDocument,
  RestoreSandboxGroupCredentialDocument,
  SandboxGroupCredentialsDocument,
  SandboxGroupCredentialVersionsDocument,
  UpdateSandboxGroupCredentialDocument,
  type CreateSandboxGroupCredentialInput,
  type SandboxGroupCredentialFieldsFragment,
  type SandboxGroupCredentialVersionFieldsFragment,
  type UpdateSandboxGroupCredentialInput,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  hasGraphQLErrorCode,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export type SandboxGroupCredential = SandboxGroupCredentialFieldsFragment;
export type SandboxGroupCredentialVersion =
  SandboxGroupCredentialVersionFieldsFragment;

const CODE_EXISTS = "SANDBOX_CREDENTIAL_ALREADY_EXISTS";
const CODE_BUSY = "SANDBOX_CREDENTIAL_CONFLICT";
const CODE_UNAVAILABLE = "SANDBOX_CREDENTIALS_UNAVAILABLE";
const CODE_VERSION_NOT_FOUND = "SANDBOX_CREDENTIAL_VERSION_NOT_FOUND";

export function useSandboxGroupCredentials(
  sandboxGroupId: string,
  ownerId?: string | null,
) {
  const { currentWorkspaceId } = useWorkspace();
  const effectiveOwnerId = ownerId ?? currentWorkspaceId;
  const { data, loading, error, refetch } = useQuery(
    SandboxGroupCredentialsDocument,
    {
      variables: {
        sandboxGroupId,
        ownerId: effectiveOwnerId ?? undefined,
      },
      skip: !sandboxGroupId,
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
    },
  );

  const credentials: SandboxGroupCredential[] =
    data?.sandboxGroupCredentials ?? [];

  return {
    credentials,
    loading: loading && data === undefined,
    error,
    unavailable: hasGraphQLErrorCode(error, CODE_UNAVAILABLE),
    refetch,
  };
}

export function useSandboxGroupCredentialVersions(
  sandboxGroupId: string,
  credentialId: string | null,
  ownerId?: string | null,
) {
  const { currentWorkspaceId } = useWorkspace();
  const effectiveOwnerId = ownerId ?? currentWorkspaceId;
  const { data, loading, error, refetch } = useQuery(
    SandboxGroupCredentialVersionsDocument,
    {
      variables: {
        sandboxGroupId,
        id: credentialId ?? "",
        ownerId: effectiveOwnerId ?? undefined,
      },
      skip: !sandboxGroupId || !credentialId,
      fetchPolicy: "network-only",
      errorPolicy: "all",
    },
  );

  const versions: SandboxGroupCredentialVersion[] =
    data?.sandboxGroupCredentialVersions ?? [];

  return {
    versions,
    loading: loading && data === undefined,
    error,
    refetch,
  };
}

export interface CredentialMutationResult<T> {
  ok: boolean;
  data: T | null;
  errorMessage: string | null;
}

export function useSandboxGroupCredentialMutations(
  sandboxGroupId: string,
  ownerId?: string | null,
  onChanged?: () => void,
) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const effectiveOwnerId = ownerId ?? currentWorkspaceId;

  const [createMut] = useMutation(CreateSandboxGroupCredentialDocument);
  const [updateMut] = useMutation(UpdateSandboxGroupCredentialDocument);
  const [deleteMut] = useMutation(DeleteSandboxGroupCredentialDocument);
  const [restoreMut] = useMutation(RestoreSandboxGroupCredentialDocument);

  const [busyId, setBusyId] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const formatError = useCallback(
    (err: unknown, fallback: string, credentialId?: string): string => {
      if (hasGraphQLErrorCode(err, CODE_EXISTS) && credentialId) {
        return t("sandboxGroups.credErrorExists", { id: credentialId });
      }
      if (hasGraphQLErrorCode(err, CODE_BUSY)) {
        return t("sandboxGroups.credErrorBusy");
      }
      if (hasGraphQLErrorCode(err, CODE_UNAVAILABLE)) {
        return t("sandboxGroups.credErrorUnavailable");
      }
      if (hasGraphQLErrorCode(err, CODE_VERSION_NOT_FOUND)) {
        return t("sandboxGroups.credErrorVersionNotFound");
      }
      return mutationErrorMessage(err, fallback);
    },
    [t],
  );

  const createCredential = useCallback(
    async (
      input: CreateSandboxGroupCredentialInput,
    ): Promise<CredentialMutationResult<SandboxGroupCredential>> => {
      setSubmitting(true);
      try {
        const res = await createMut({
          variables: {
            sandboxGroupId,
            input,
            ownerId: effectiveOwnerId ?? undefined,
          },
        });
        const created = res.data?.createSandboxGroupCredential ?? null;
        if (!created) {
          throw new Error("createSandboxGroupCredential returned empty result");
        }
        toast.success(t("sandboxGroups.credCreatedToast", { id: created.id }));
        onChanged?.();
        return { ok: true, data: created, errorMessage: null };
      } catch (err) {
        const message = formatError(
          err,
          t("sandboxGroups.credCreateError"),
          input.id,
        );
        return { ok: false, data: null, errorMessage: message };
      } finally {
        setSubmitting(false);
      }
    },
    [createMut, effectiveOwnerId, formatError, onChanged, sandboxGroupId, t],
  );

  const updateCredential = useCallback(
    async (
      id: string,
      input: UpdateSandboxGroupCredentialInput,
      options?: { silentToast?: boolean },
    ): Promise<CredentialMutationResult<SandboxGroupCredential>> => {
      setSubmitting(true);
      setBusyId(id);
      try {
        const res = await updateMut({
          variables: {
            sandboxGroupId,
            id,
            input,
            ownerId: effectiveOwnerId ?? undefined,
          },
        });
        const updated = res.data?.updateSandboxGroupCredential ?? null;
        if (!updated) {
          throw new Error("updateSandboxGroupCredential returned empty result");
        }
        if (!options?.silentToast) {
          toast.success(t("sandboxGroups.credUpdatedToast", { id }));
        }
        onChanged?.();
        return { ok: true, data: updated, errorMessage: null };
      } catch (err) {
        if (options?.silentToast) {
          toast.error(formatError(err, t("sandboxGroups.credUpdateError"), id));
        }
        return {
          ok: false,
          data: null,
          errorMessage: formatError(err, t("sandboxGroups.credUpdateError"), id),
        };
      } finally {
        setSubmitting(false);
        setBusyId(null);
      }
    },
    [effectiveOwnerId, formatError, onChanged, sandboxGroupId, t, updateMut],
  );

  const deleteCredential = useCallback(
    async (id: string): Promise<boolean> => {
      setBusyId(id);
      try {
        await deleteMut({
          variables: {
            sandboxGroupId,
            id,
            ownerId: effectiveOwnerId ?? undefined,
          },
        });
        toast.success(t("sandboxGroups.credDeletedToast", { id }));
        onChanged?.();
        return true;
      } catch (err) {
        toast.error(formatError(err, t("sandboxGroups.credDeleteError"), id));
        return false;
      } finally {
        setBusyId(null);
      }
    },
    [deleteMut, effectiveOwnerId, formatError, onChanged, sandboxGroupId, t],
  );

  const restoreCredential = useCallback(
    async (
      id: string,
      versionId: number,
    ): Promise<SandboxGroupCredential | null> => {
      setBusyId(id);
      try {
        const res = await restoreMut({
          variables: {
            sandboxGroupId,
            id,
            versionId,
            ownerId: effectiveOwnerId ?? undefined,
          },
        });
        const restored = res.data?.restoreSandboxGroupCredential ?? null;
        if (!restored) {
          throw new Error("restoreSandboxGroupCredential returned empty result");
        }
        toast.success(
          t("sandboxGroups.credRestoredToast", {
            id,
            version: versionId,
          }),
        );
        onChanged?.();
        return restored;
      } catch (err) {
        toast.error(formatError(err, t("sandboxGroups.credRestoreError"), id));
        return null;
      } finally {
        setBusyId(null);
      }
    },
    [effectiveOwnerId, formatError, onChanged, restoreMut, sandboxGroupId, t],
  );

  return {
    createCredential,
    updateCredential,
    deleteCredential,
    restoreCredential,
    busyId,
    submitting,
  };
}
