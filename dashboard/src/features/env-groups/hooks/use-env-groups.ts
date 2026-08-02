import { useCallback, useState } from "react";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CreateEnvGroupDocument,
  DeleteEnvGroupDocument,
  DeleteEnvGroupSecretFileDocument,
  DeleteEnvGroupVarDocument,
  EnvGroupDocument,
  EnvGroupSecretFileContentDocument,
  EnvGroupsDocument,
  EnvGroupVarValueDocument,
  LinkEnvGroupDocument,
  RenameEnvGroupDocument,
  SetEnvGroupSecretFileDocument,
  SetEnvGroupVarDocument,
  SetEnvGroupVarsDocument,
  UnlinkEnvGroupDocument,
} from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { EnvGroupQuery, EnvGroupsQuery } from "@/graphql/definitions";
import type {
  CreateEnvGroupInput,
  EnvGroupView,
} from "@/features/env-groups/types";
import type { EnvVarKey, SecretFileName } from "@/features/services/types";

type RawGroup = NonNullable<NonNullable<EnvGroupsQuery["envGroups"]>[number]>;

/** Maps bex-api's deeply nullable EnvGroup wire shape to the UI view. */
export function mapEnvGroup(
  raw: RawGroup | NonNullable<EnvGroupQuery["envGroup"]> | null | undefined,
): EnvGroupView | null {
  if (!raw?.id || !raw.name) return null;
  return {
    id: raw.id,
    name: raw.name,
    ownerId: raw.ownerId ?? null,
    createdAt: raw.createdAt ?? null,
    updatedAt: raw.updatedAt ?? null,
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
  raw: EnvGroupsQuery["envGroups"] | undefined,
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
    fetchPolicy: "cache-and-network",
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
    loading: !resolved || loading,
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
  busy: boolean;
}

/** Group lifecycle and service-link mutations shared by both dashboard surfaces. */
export function useEnvGroupMutations(
  refetch?: Refetch,
  options: { skipDeleteRefetch?: boolean; skipRenameRefetch?: boolean } = {},
): UseEnvGroupMutationsResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [createEnvGroup] = useMutation(CreateEnvGroupDocument);
  const [renameEnvGroup] = useMutation(RenameEnvGroupDocument);
  const [deleteEnvGroup] = useMutation(DeleteEnvGroupDocument);
  const [linkEnvGroup] = useMutation(LinkEnvGroupDocument);
  const [unlinkEnvGroup] = useMutation(UnlinkEnvGroupDocument);
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
      } catch {
        toast.error(t("envGroups.createError", { name: input.name }));
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
      } catch {
        toast.error(t("envGroups.renameError"));
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
        if (!options.skipDeleteRefetch) {
          await bestEffortRefetch(refetch);
        }
        toast.success(t("envGroups.deleteSuccess"));
        return true;
      } catch {
        toast.error(t("envGroups.deleteError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [deleteEnvGroup, options.skipDeleteRefetch, refetch, t],
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

  return {
    createGroup,
    renameGroup,
    deleteGroup,
    linkGroup,
    unlinkGroup,
    busy,
  };
}

export function useRevealEnvGroupVar(groupId: string) {
  const client = useApolloClient();
  return useCallback(
    async (key: string): Promise<string> => {
      const result = await client.query({
        query: EnvGroupVarValueDocument,
        variables: { id: groupId, key },
        fetchPolicy: "network-only",
        errorPolicy: "none",
      });
      return result.data?.envGroupVar?.value ?? "";
    },
    [client, groupId],
  );
}

export function useEnvGroupVarMutations(groupId: string, refetch: Refetch) {
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
        toast.success(t("envGroups.varsSaveSuccess"), {
          description: t("envGroups.rolloutNote"),
        });
        return true;
      } catch {
        toast.error(t("envGroups.varsSaveError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [groupId, refetch, setEnvGroupVars, t],
  );

  const setVar = useCallback(
    async (key: string, value: string) => {
      setBusy(true);
      try {
        await setEnvGroupVar({ variables: { id: groupId, key, value } });
        await bestEffortRefetch(refetch);
        toast.success(t("envGroups.varSaveSuccess", { key }), {
          description: t("envGroups.rolloutNote"),
        });
        return true;
      } catch {
        toast.error(t("envGroups.varSaveError", { key }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [groupId, refetch, setEnvGroupVar, t],
  );

  const deleteVar = useCallback(
    async (key: string) => {
      setBusy(true);
      try {
        await deleteEnvGroupVar({ variables: { id: groupId, key } });
        await bestEffortRefetch(refetch);
        toast.success(t("envGroups.varDeleteSuccess", { key }), {
          description: t("envGroups.rolloutNote"),
        });
        return true;
      } catch {
        toast.error(t("envGroups.varDeleteError", { key }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [deleteEnvGroupVar, groupId, refetch, t],
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
        fetchPolicy: "network-only",
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
        toast.success(t("envGroups.fileSaveSuccess", { name }), {
          description: t("envGroups.rolloutNote"),
        });
        return true;
      } catch {
        toast.error(t("envGroups.fileSaveError", { name }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [groupId, refetch, setEnvGroupSecretFile, t],
  );

  const deleteFile = useCallback(
    async (name: string) => {
      setBusy(true);
      try {
        await deleteEnvGroupSecretFile({ variables: { id: groupId, name } });
        await bestEffortRefetch(refetch);
        toast.success(t("envGroups.fileDeleteSuccess", { name }), {
          description: t("envGroups.rolloutNote"),
        });
        return true;
      } catch {
        toast.error(t("envGroups.fileDeleteError", { name }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [deleteEnvGroupSecretFile, groupId, refetch, t],
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
