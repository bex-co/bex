import { useCallback, useState } from "react";
import { useQuery, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  EnvGroupsDocument,
  CreateEnvGroupDocument,
  DeleteEnvGroupDocument,
  LinkEnvGroupDocument,
  UnlinkEnvGroupDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import type { EnvGroupsQuery } from "@/graphql/definitions";
import type { EnvGroupView } from "@/features/services/types";

// bex-api's env-groups GraphQL: a group is a reusable bundle of env vars + secret
// files (docs/bex-api.md) that can be linked to many services. The list is
// service-independent (`envGroups`); linking/unlinking attaches a group to a
// specific service. Every link/unlink rolls the affected service's pods, so the
// toast says the service is redeploying.

type RawGroup = NonNullable<
  NonNullable<EnvGroupsQuery["envGroups"]>[number]
>;

/** Maps bex-api's deeply-nullable EnvGroup wire shape to the flat view the UI uses. */
function mapGroup(raw: RawGroup): EnvGroupView | null {
  if (raw.id == null || raw.name == null) return null;
  return {
    id: raw.id,
    name: raw.name,
    serviceLinks: (raw.serviceLinks ?? []).filter(
      (s): s is string => s != null,
    ),
    envVarKeys: (raw.envVars ?? [])
      .map((v) => v?.key)
      .filter((k): k is string => k != null),
    secretFileNames: (raw.secretFiles ?? [])
      .map((f) => f?.name)
      .filter((n): n is string => n != null),
  };
}

function mapGroups(
  raw: EnvGroupsQuery["envGroups"] | undefined,
): EnvGroupView[] {
  return (raw ?? [])
    .filter((g): g is RawGroup => g != null)
    .map(mapGroup)
    .filter((g): g is EnvGroupView => g != null);
}

export interface UseEnvGroupsResult {
  groups: EnvGroupView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the groups query, resolving to the fresh list. */
  refetch: () => Promise<EnvGroupView[]>;
}

/**
 * Reads all environment groups (`envGroups{ id name serviceLinks envVars{ key }
 * secretFiles{ name } }`). Service-independent — membership of the current service
 * is derived from each group's `serviceLinks`.
 */
export function useEnvGroups(): UseEnvGroupsResult {
  const { data, loading, error, refetch } = useQuery(EnvGroupsDocument, {
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const refetchGroups = useCallback(async () => {
    const res = await refetch();
    return mapGroups(res.data?.envGroups);
  }, [refetch]);

  return {
    groups: mapGroups(data?.envGroups),
    loading,
    error,
    refetch: refetchGroups,
  };
}

export interface UseEnvGroupMutationsResult {
  /** Create a new (empty) group; resolves true on success. */
  createGroup: (name: string) => Promise<boolean>;
  /** Delete a group; resolves true on success. */
  deleteGroup: (id: string) => Promise<boolean>;
  /** Attach a group to a service; resolves true on success. */
  linkGroup: (id: string, serviceId: string) => Promise<boolean>;
  /** Detach a group from a service; resolves true on success. */
  unlinkGroup: (id: string, serviceId: string) => Promise<boolean>;
  /** A write is in flight (disable the form while true). */
  busy: boolean;
}

/**
 * Wires the env-group write mutations (`createEnvGroup` / `deleteEnvGroup` /
 * `linkEnvGroup` / `unlinkEnvGroup`), refetching the groups after each write and
 * toasting the result. Link/unlink roll the affected service's pods, so those
 * success toasts carry the redeploy note.
 */
export function useEnvGroupMutations(
  refetch: () => Promise<EnvGroupView[]>,
): UseEnvGroupMutationsResult {
  const { t } = useTranslations();
  const [createEnvGroup] = useMutation(CreateEnvGroupDocument);
  const [deleteEnvGroup] = useMutation(DeleteEnvGroupDocument);
  const [linkEnvGroup] = useMutation(LinkEnvGroupDocument);
  const [unlinkEnvGroup] = useMutation(UnlinkEnvGroupDocument);
  const [busy, setBusy] = useState(false);

  const createGroup = useCallback(
    async (name: string) => {
      setBusy(true);
      try {
        await createEnvGroup({ variables: { name } });
        await refetch();
        toast.success(t("services.envGroupCreateSuccess", { name }));
        return true;
      } catch {
        toast.error(t("services.envGroupCreateError", { name }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [createEnvGroup, refetch, t],
  );

  const deleteGroup = useCallback(
    async (id: string) => {
      setBusy(true);
      try {
        await deleteEnvGroup({ variables: { id } });
        await refetch();
        toast.success(t("services.envGroupDeleteSuccess"));
        return true;
      } catch {
        toast.error(t("services.envGroupDeleteError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [deleteEnvGroup, refetch, t],
  );

  const linkGroup = useCallback(
    async (id: string, serviceId: string) => {
      setBusy(true);
      try {
        await linkEnvGroup({ variables: { id, serviceId } });
        await refetch();
        toast.success(t("services.envGroupLinkSuccess"), {
          description: t("services.envRolloutNote"),
        });
        return true;
      } catch {
        toast.error(t("services.envGroupLinkError"));
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
        await refetch();
        toast.success(t("services.envGroupUnlinkSuccess"), {
          description: t("services.envRolloutNote"),
        });
        return true;
      } catch {
        toast.error(t("services.envGroupUnlinkError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [unlinkEnvGroup, refetch, t],
  );

  return { createGroup, deleteGroup, linkGroup, unlinkGroup, busy };
}

/**
 * Classifies an env-groups GraphQL error into the states the panel renders
 * differently: the store being unconfigured (503-equivalent) and a permission
 * denial (403-equivalent) both come back as GraphQL errors whose message carries
 * bex-api's sentinel text. Same logic as classifyEnvVarError.
 */
export type EnvGroupErrorKind = "unavailable" | "forbidden" | "generic";

export function classifyEnvGroupError(
  error: Error | undefined,
): EnvGroupErrorKind | null {
  if (!error) return null;
  const msg = error.message.toLowerCase();
  if (msg.includes("secret store")) return "unavailable";
  if (msg.includes("forbidden")) return "forbidden";
  return "generic";
}
