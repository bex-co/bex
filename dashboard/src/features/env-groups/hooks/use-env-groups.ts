import { useCallback, useEffect, useRef, useState } from "react";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CreateEnvGroupDocument,
  CloneEnvGroupDocument,
  DeleteEnvGroupDocument,
  DeleteEnvGroupSecretFileDocument,
  DeleteEnvGroupVarDocument,
  EnvGroupDocument,
  EnvGroupSecretFileContentDocument,
  EnvGroupVarValueDocument,
  EnvGroupsDocument,
  LinkEnvGroupDocument,
  MoveEnvGroupDocument,
  PatchEnvGroupEnvironmentDocument,
  RenameEnvGroupDocument,
  SetEnvGroupSecretFileDocument,
  SetEnvGroupVarDocument,
  SetEnvGroupVarsDocument,
  UnlinkEnvGroupDocument,
  type EnvGroupQuery,
  type PatchEnvGroupEnvironmentMutationVariables,
} from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { PRIMED_FETCH_POLICY } from "@/common/lib/fetch-policy";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type {
  CreateEnvGroupInput,
  EnvGroupView,
} from "@/features/env-groups/types";
import type { EnvVarKey, SecretFileName } from "@/features/services/types";
import type { EnvironmentPatchInput } from "@/features/services/lib/environment-draft";

type RawGroup = NonNullable<EnvGroupQuery["envGroup"]>;

/** Maps bex-api's deeply nullable EnvGroup wire shape to the UI view. */
export function mapEnvGroup(
  raw: RawGroup | null | undefined,
): EnvGroupView | null {
  if (!raw?.id || !raw.name) return null;
  return {
    id: raw.id,
    name: raw.name,
    ownerId: raw.ownerId ?? null,
    environmentId: raw.environmentId ?? null,
    createdAt: raw.createdAt ?? null,
    updatedAt: raw.updatedAt ?? null,
    revision: raw.revision ?? null,
    serviceLinks: (raw.serviceLinks ?? []).filter(
      (serviceId): serviceId is string => serviceId != null,
    ),
    envVarKeys: (raw.envVars ?? [])
      .map((variable) => variable?.key)
      .filter((key): key is string => key != null),
    secretFileNames: (raw.secretFiles ?? [])
      .map((file) => file?.name)
      .filter((name): name is string => name != null),
  };
}

function mapEnvGroups(
  raw: Array<RawGroup | null> | null | undefined,
): EnvGroupView[] {
  return (raw ?? [])
    .map((group) => mapEnvGroup(group))
    .filter((group): group is EnvGroupView => group != null);
}

export interface UseEnvGroupsResult {
  groups: EnvGroupView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<EnvGroupView[]>;
}

/**
 * Reads every environment group in the switcher-selected workspace (w6/m24):
 * skipped until the selection resolves to an id, mirroring useApiKeys/useServices.
 */
export function useEnvGroups(): UseEnvGroupsResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch } = useQuery(EnvGroupsDocument, {
    variables: { ownerId: currentWorkspaceId },
    skip: !resolved,
    fetchPolicy: PRIMED_FETCH_POLICY,
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });

  const refetchGroups = useCallback(async () => {
    const result = await refetch();
    return mapEnvGroups(result.data?.envGroups);
  }, [refetch]);

  return {
    groups: mapEnvGroups(data?.envGroups),
    // cache-and-network reports `loading` again while refreshing a cached
    // result. Keep rendering that result — especially the empty list after a
    // delete — and reserve the page skeleton for the true first load.
    loading: !resolved || (loading && data === undefined),
    error,
    refetch: refetchGroups,
  };
}

export interface UseEnvGroupResult {
  group: EnvGroupView | null;
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<EnvGroupView | null>;
}

/** Reads one group by id, including keys/names and linked service ids. */
export function useEnvGroup(id: string): UseEnvGroupResult {
  const { data, loading, error, refetch } = useQuery(EnvGroupDocument, {
    variables: { id },
    fetchPolicy: "cache-first",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });

  const refetchGroup = useCallback(async () => {
    const result = await refetch();
    return mapEnvGroup(result.data?.envGroup);
  }, [refetch]);

  return {
    group: mapEnvGroup(data?.envGroup),
    loading,
    error,
    refetch: refetchGroup,
  };
}

type Refetch = () => Promise<unknown>;

async function bestEffortRefetch(refetch: Refetch | undefined) {
  try {
    await refetch?.();
  } catch {
    // The write already succeeded. A cache refresh failure must not report the
    // mutation itself as failed; cache-and-network queries retry on next mount.
  }
}

export interface UseEnvGroupMutationsResult {
  createGroup: (input: CreateEnvGroupInput) => Promise<string | null>;
  renameGroup: (id: string, name: string) => Promise<boolean>;
  deleteGroup: (id: string) => Promise<boolean>;
  linkGroup: (id: string, serviceId: string) => Promise<boolean>;
  unlinkGroup: (id: string, serviceId: string) => Promise<boolean>;
  moveGroup: (id: string, environmentId: string | null) => Promise<boolean>;
  cloneGroup: (
    id: string,
    name: string,
    ownerId: string,
    environmentId: string | null,
  ) => Promise<string | null>;
  busy: boolean;
}

/** Group lifecycle and service-link mutations shared by both dashboard surfaces. */
export function useEnvGroupMutations(
  refetch?: Refetch,
  options: { skipRenameRefetch?: boolean } = {},
): UseEnvGroupMutationsResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [createEnvGroup] = useMutation(CreateEnvGroupDocument);
  const [renameEnvGroup] = useMutation(RenameEnvGroupDocument);
  const [deleteEnvGroup] = useMutation(DeleteEnvGroupDocument);
  const [linkEnvGroup] = useMutation(LinkEnvGroupDocument);
  const [unlinkEnvGroup] = useMutation(UnlinkEnvGroupDocument);
  const [moveEnvGroup] = useMutation(MoveEnvGroupDocument);
  const [cloneEnvGroup] = useMutation(CloneEnvGroupDocument);
  const client = useApolloClient();
  const [busy, setBusy] = useState(false);

  const createGroup = useCallback(
    async (input: CreateEnvGroupInput) => {
      // Scoped to the switcher's selected workspace (w6/m24) — refused (never
      // sent with a null ownerId, which the backend would silently route to
      // the caller's default workspace) until the workspace list resolves,
      // mirroring useCreateApiKey.
      if (currentWorkspaceId == null) {
        toast.error(t("envGroups.createError", { name: input.name }));
        return null;
      }
      setBusy(true);
      try {
        const result = await createEnvGroup({
          variables: {
            name: input.name,
            ownerId: currentWorkspaceId,
            envVars: input.envVars,
            secretFiles: input.secretFiles,
            serviceIds: input.serviceIds,
          },
        });
        const id = result.data?.createEnvGroup?.id ?? null;
        if (!id) throw new Error("createEnvGroup returned no id");
        await bestEffortRefetch(refetch);
        toast.success(t("envGroups.createSuccess", { name: input.name }));
        return id;
      } catch (error) {
        toast.error(t("envGroups.createError", { name: input.name }), {
          description: error instanceof Error ? error.message : undefined,
        });
        return null;
      } finally {
        setBusy(false);
      }
    },
    [createEnvGroup, currentWorkspaceId, refetch, t],
  );

  const renameGroup = useCallback(
    async (id: string, name: string) => {
      setBusy(true);
      try {
        await renameEnvGroup({ variables: { id, name } });
        if (!options.skipRenameRefetch) {
          await bestEffortRefetch(refetch);
        }
        toast.success(t("envGroups.renameSuccess", { name }));
        return true;
      } catch (error) {
        toast.error(t("envGroups.renameError"), {
          description: error instanceof Error ? error.message : undefined,
        });
        return false;
      } finally {
        setBusy(false);
      }
    },
    [options.skipRenameRefetch, refetch, renameEnvGroup, t],
  );

  const deleteGroup = useCallback(
    async (id: string) => {
      setBusy(true);
      try {
        await deleteEnvGroup({ variables: { id } });
        client.cache.modify({
          fields: {
            envGroups(existing, { readField }) {
              if (!Array.isArray(existing)) return existing;
              return existing.filter(
                (reference) =>
                  readField("id", reference as { __ref: string }) !== id,
              );
            },
          },
        });
        const cacheID = client.cache.identify({ __typename: "EnvGroup", id });
        if (cacheID) client.cache.evict({ id: cacheID });
        client.cache.gc();
        await bestEffortRefetch(refetch);
        toast.success(t("envGroups.deleteSuccess"));
        return true;
      } catch {
        toast.error(t("envGroups.deleteError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [client, deleteEnvGroup, refetch, t],
  );

  const linkGroup = useCallback(
    async (id: string, serviceId: string) => {
      setBusy(true);
      try {
        await linkEnvGroup({ variables: { id, serviceId } });
        await bestEffortRefetch(refetch);
        toast.success(t("envGroups.linkSuccess"), {
          description: t("envGroups.rolloutNote"),
        });
        return true;
      } catch {
        toast.error(t("envGroups.linkError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [linkEnvGroup, refetch, t],
  );

  const unlinkGroup = useCallback(
    async (id: string, serviceId: string) => {
      setBusy(true);
      try {
        await unlinkEnvGroup({ variables: { id, serviceId } });
        await bestEffortRefetch(refetch);
        toast.success(t("envGroups.unlinkSuccess"), {
          description: t("envGroups.rolloutNote"),
        });
        return true;
      } catch {
        toast.error(t("envGroups.unlinkError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [refetch, t, unlinkEnvGroup],
  );

  const moveGroup = useCallback(
    async (id: string, environmentId: string | null) => {
      setBusy(true);
      try {
        await moveEnvGroup({
          variables: { id, environmentId: environmentId ?? "" },
        });
        // Environment reads also carry envGroupIds. Evict every argument-keyed
        // page so active watchers and the next Project visit cannot retain the
        // old source/target memberships after the normalized group updates.
        client.cache.evict({ id: "ROOT_QUERY", fieldName: "environments" });
        client.cache.gc();
        await bestEffortRefetch(refetch);
        toast.success(t("envGroups.moveSuccess"));
        return true;
      } catch (error) {
        toast.error(t("envGroups.moveError"), {
          description: error instanceof Error ? error.message : undefined,
        });
        return false;
      } finally {
        setBusy(false);
      }
    },
    [client, moveEnvGroup, refetch, t],
  );

  const cloneGroup = useCallback(
    async (
      id: string,
      name: string,
      ownerId: string,
      environmentId: string | null,
    ) => {
      setBusy(true);
      try {
        const result = await cloneEnvGroup({
          variables: { id, name, ownerId, environmentId },
        });
        const cloneID = result.data?.cloneEnvGroup?.id ?? null;
        if (!cloneID) throw new Error("cloneEnvGroup returned no id");
        // The mutation returns the new object but does not append it to an
        // argument-keyed target list. Force that workspace's next list read to
        // the network before switching/navigating there.
        client.cache.evict({
          id: "ROOT_QUERY",
          fieldName: "envGroups",
          args: { ownerId },
        });
        client.cache.gc();
        if (ownerId === currentWorkspaceId) {
          await bestEffortRefetch(refetch);
        }
        toast.success(t("envGroups.cloneSuccess", { name }));
        return cloneID;
      } catch (error) {
        toast.error(t("envGroups.cloneError"), {
          description: error instanceof Error ? error.message : undefined,
        });
        return null;
      } finally {
        setBusy(false);
      }
    },
    [client, cloneEnvGroup, currentWorkspaceId, refetch, t],
  );

  return {
    createGroup,
    renameGroup,
    deleteGroup,
    linkGroup,
    unlinkGroup,
    moveGroup,
    cloneGroup,
    busy,
  };
}

export type EnvGroupSaveMode = "save_only" | "deploy" | "rebuild";

export function useEnvGroupEnvironmentPatch(
  groupId: string,
  revision: string | null,
  refetch: Refetch,
) {
  const [mutate, { loading }] = useMutation(PatchEnvGroupEnvironmentDocument);
  const revisionRef = useRef(revision);
  useEffect(() => {
    revisionRef.current = revision;
  }, [groupId, revision]);

  const save = useCallback(
    async (patch: EnvironmentPatchInput, saveMode: EnvGroupSaveMode) => {
      const variables: PatchEnvGroupEnvironmentMutationVariables = {
        id: groupId,
        ...patch,
        saveMode,
        expectedRevision: revisionRef.current,
      };
      const result = await mutate({ variables });
      const saved = result.data?.patchEnvGroupEnvironment;
      if (!saved?.revision) {
        throw new Error("environment group patch returned no revision");
      }
      revisionRef.current = saved.revision;
      await bestEffortRefetch(refetch);
      return {
        ...saved,
        affectedServiceIds: (saved.affectedServiceIds ?? []).filter(
          (id): id is string => id != null,
        ),
        failedServiceIds: (saved.failedServiceIds ?? []).filter(
          (id): id is string => id != null,
        ),
      };
    },
    [groupId, mutate, refetch],
  );

  const retryRollout = useCallback(
    async (saveMode: Exclude<EnvGroupSaveMode, "save_only">) => {
      const result = await save({ envVars: [], secretFiles: [] }, saveMode);
      return result.failedServiceIds.length === 0;
    },
    [save],
  );

  return { save, retryRollout, saving: loading };
}

export function useRevealEnvGroupVar(groupId: string) {
  const client = useApolloClient();
  return useCallback(
    async (key: string): Promise<string> => {
      const result = await client.query({
        query: EnvGroupVarValueDocument,
        variables: { id: groupId, key },
        fetchPolicy: "no-cache",
        errorPolicy: "none",
      });
      return result.data?.envGroupVar?.value ?? "";
    },
    [client, groupId],
  );
}

export function useEnvGroupVarMutations(
  groupId: string,
  refetch: Refetch,
  hasLinkedServices = false,
) {
  const { t } = useTranslations();
  const [setEnvGroupVars] = useMutation(SetEnvGroupVarsDocument);
  const [setEnvGroupVar] = useMutation(SetEnvGroupVarDocument);
  const [deleteEnvGroupVar] = useMutation(DeleteEnvGroupVarDocument);
  const [busy, setBusy] = useState(false);

  const setVars = useCallback(
    async (envVars: Array<{ key: string; value: string }>) => {
      setBusy(true);
      try {
        await setEnvGroupVars({ variables: { id: groupId, envVars } });
        await bestEffortRefetch(refetch);
        if (hasLinkedServices) {
          toast.success(t("envGroups.varsSaveSuccess"), {
            description: t("envGroups.rolloutNote"),
          });
        } else toast.success(t("envGroups.varsSaveSuccess"));
        return true;
      } catch {
        toast.error(t("envGroups.varsSaveError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [groupId, hasLinkedServices, refetch, setEnvGroupVars, t],
  );

  const setVar = useCallback(
    async (key: string, value: string, generateValue?: boolean) => {
      setBusy(true);
      try {
        await setEnvGroupVar({
          variables: {
            id: groupId,
            key,
            value: generateValue ? undefined : value,
            generateValue: generateValue || undefined,
          },
        });
        await bestEffortRefetch(refetch);
        if (hasLinkedServices) {
          toast.success(t("envGroups.varSaveSuccess", { key }), {
            description: t("envGroups.rolloutNote"),
          });
        } else toast.success(t("envGroups.varSaveSuccess", { key }));
        return true;
      } catch {
        toast.error(t("envGroups.varSaveError", { key }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [groupId, hasLinkedServices, refetch, setEnvGroupVar, t],
  );

  const deleteVar = useCallback(
    async (key: string) => {
      setBusy(true);
      try {
        await deleteEnvGroupVar({ variables: { id: groupId, key } });
        await bestEffortRefetch(refetch);
        if (hasLinkedServices) {
          toast.success(t("envGroups.varDeleteSuccess", { key }), {
            description: t("envGroups.rolloutNote"),
          });
        } else toast.success(t("envGroups.varDeleteSuccess", { key }));
        return true;
      } catch {
        toast.error(t("envGroups.varDeleteError", { key }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [deleteEnvGroupVar, groupId, hasLinkedServices, refetch, t],
  );

  return { setVars, setVar, deleteVar, busy };
}

export function useRevealEnvGroupSecretFile(groupId: string) {
  const client = useApolloClient();
  return useCallback(
    async (name: string): Promise<string> => {
      const result = await client.query({
        query: EnvGroupSecretFileContentDocument,
        variables: { id: groupId, name },
        fetchPolicy: "no-cache",
        errorPolicy: "none",
      });
      return result.data?.envGroupSecretFile?.content ?? "";
    },
    [client, groupId],
  );
}

export function useEnvGroupSecretFileMutations(
  groupId: string,
  refetch: Refetch,
  hasLinkedServices = false,
) {
  const { t } = useTranslations();
  const [setEnvGroupSecretFile] = useMutation(SetEnvGroupSecretFileDocument);
  const [deleteEnvGroupSecretFile] = useMutation(
    DeleteEnvGroupSecretFileDocument,
  );
  const [busy, setBusy] = useState(false);

  const setFile = useCallback(
    async (name: string, content: string) => {
      setBusy(true);
      try {
        await setEnvGroupSecretFile({
          variables: { id: groupId, name, content },
        });
        await bestEffortRefetch(refetch);
        if (hasLinkedServices) {
          toast.success(t("envGroups.fileSaveSuccess", { name }), {
            description: t("envGroups.rolloutNote"),
          });
        } else toast.success(t("envGroups.fileSaveSuccess", { name }));
        return true;
      } catch {
        toast.error(t("envGroups.fileSaveError", { name }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [groupId, hasLinkedServices, refetch, setEnvGroupSecretFile, t],
  );

  const deleteFile = useCallback(
    async (name: string) => {
      setBusy(true);
      try {
        await deleteEnvGroupSecretFile({ variables: { id: groupId, name } });
        await bestEffortRefetch(refetch);
        if (hasLinkedServices) {
          toast.success(t("envGroups.fileDeleteSuccess", { name }), {
            description: t("envGroups.rolloutNote"),
          });
        } else toast.success(t("envGroups.fileDeleteSuccess", { name }));
        return true;
      } catch {
        toast.error(t("envGroups.fileDeleteError", { name }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [deleteEnvGroupSecretFile, groupId, hasLinkedServices, refetch, t],
  );

  return { setFile, deleteFile, busy };
}

export function envVarKeys(group: EnvGroupView | null): EnvVarKey[] {
  return (group?.envVarKeys ?? []).map((key) => ({ id: key, key }));
}

export function secretFileNames(group: EnvGroupView | null): SecretFileName[] {
  return (group?.secretFileNames ?? []).map((name) => ({ id: name, name }));
}

export type EnvGroupErrorKind = "unavailable" | "forbidden" | "generic";

export function isEnvGroupNotFound(error: Error | undefined): boolean {
  return error?.message.toLowerCase().includes("not found") ?? false;
}

export function classifyEnvGroupError(
  error: Error | undefined,
): EnvGroupErrorKind | null {
  if (!error) return null;
  const message = error.message.toLowerCase();
  if (message.includes("secret store")) return "unavailable";
  if (message.includes("forbidden")) return "forbidden";
  return "generic";
}
