import { useCallback, useState } from "react";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  DatabaseIpAllowListDocument,
  DatabaseUsersDocument,
  SetDatabaseIpAllowListDocument,
  CreateDatabaseUserDocument,
  DeleteDatabaseUserDocument,
  DatabasePooledConnectionDocument,
} from "@/graphql/definitions";
import type { IPAllowListEntryDraft } from "@/common/lib/ip-allow-list";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { useTranslations } from "@/common/hooks/use-translations";

export interface PooledStrings {
  internal: string;
  external: string;
}

/**
 * Reads + edits a database's access controls: the external-endpoint IP allowlist
 * and the additional Postgres login roles, plus an on-demand reveal of the
 * pooled connection strings (only surfaced on request, like the password). Every
 * mutation refetches its list so the panel reflects the operator, and a new
 * user's generated password is returned once (never re-fetchable).
 */
export function useAccessControl(id: string) {
  const { t } = useTranslations();
  const client = useApolloClient();

  const allowListQuery = useQuery(DatabaseIpAllowListDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });
  const usersQuery = useQuery(DatabaseUsersDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });
  const [setAllowListMut, { loading: savingAllowList }] = useMutation(
    SetDatabaseIpAllowListDocument,
  );
  const [createUserMut, { loading: creatingUser }] = useMutation(
    CreateDatabaseUserDocument,
  );
  const [deleteUserMut] = useMutation(DeleteDatabaseUserDocument);

  // Entries, not bare CIDRs: each carries the operator-facing description a
  // human gave it, which persists end to end (w4/m24).
  const allowList: IPAllowListEntryDraft[] = (
    allowListQuery.data?.database?.ipAllowListEntries ?? []
  )
    .filter((e): e is NonNullable<typeof e> => e != null)
    .map((e) => ({ cidrBlock: e.cidrBlock, description: e.description ?? "" }));

  const users: string[] = (usersQuery.data?.databaseUsers ?? [])
    .filter((u): u is NonNullable<typeof u> => u != null)
    .map((u) => u.name ?? "")
    .filter((n) => n !== "");

  const saveAllowList = useCallback(
    async (entries: IPAllowListEntryDraft[]): Promise<boolean> => {
      try {
        await setAllowListMut({ variables: { id, entries } });
        toast.success(t("databases.accessAllowListSaved"));
        void allowListQuery.refetch();
        return true;
      } catch (e) {
        toast.error(
          t("databases.accessAllowListError", { error: (e as Error).message }),
        );
        return false;
      }
    },
    [setAllowListMut, allowListQuery, id, t],
  );

  const createUser = useCallback(
    async (name: string): Promise<string | null> => {
      try {
        const res = await createUserMut({ variables: { id, name } });
        void usersQuery.refetch();
        toast.success(t("databases.accessUserCreated", { name }));
        return res.data?.createDatabaseUser?.password ?? null;
      } catch (e) {
        toast.error(
          t("databases.accessUserError", { error: (e as Error).message }),
        );
        return null;
      }
    },
    [createUserMut, usersQuery, id, t],
  );

  const deleteUser = useCallback(
    async (name: string) => {
      try {
        await deleteUserMut({ variables: { id, name } });
        void usersQuery.refetch();
        toast.success(t("databases.accessUserDeleted", { name }));
      } catch {
        toast.error(t("databases.accessUserDeleteError", { name }));
      }
    },
    [deleteUserMut, usersQuery, id, t],
  );

  // Pooled connection strings, revealed on demand (never on mount).
  const [pooled, setPooled] = useState<PooledStrings | null>(null);
  const [poolLoading, setPoolLoading] = useState(false);
  const revealPooled = useCallback(async () => {
    setPoolLoading(true);
    try {
      const res = await client.query({
        query: DatabasePooledConnectionDocument,
        variables: { id },
        fetchPolicy: "network-only",
        errorPolicy: "none",
      });
      const ci = res.data?.databaseConnectionInfo;
      setPooled({
        internal: ci?.internalConnectionPoolString ?? "",
        external: ci?.externalConnectionPoolString ?? "",
      });
    } catch {
      toast.error(t("databases.accessPoolerError"));
    } finally {
      setPoolLoading(false);
    }
  }, [client, id, t]);

  return {
    allowList,
    users,
    loading: allowListQuery.loading || usersQuery.loading,
    savingAllowList,
    creatingUser,
    saveAllowList,
    createUser,
    deleteUser,
    pooled,
    poolLoading,
    revealPooled,
  };
}
