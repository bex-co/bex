import { useCallback, useState } from "react";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  DatabaseIpAllowListDocument,
  DatabaseCredentialsDocument,
  type DatabaseCredentialsQuery,
  SetDatabaseIpAllowListDocument,
  CreateDatabaseCredentialDocument,
  DeleteDatabaseCredentialDocument,
  DatabasePooledConnectionDocument,
} from "@/graphql/definitions";
import type { IPAllowListEntryDraft } from "@/common/lib/ip-allow-list";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  hasGraphQLErrorCode,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";

export interface PooledStrings {
  internal: string;
  external: string;
}

/**
 * Reads + edits a database's access controls: the external-endpoint IP allowlist
 * and Render's credentials (w5/m265 — a new credential becomes the default and
 * its password is read through the connection info, never echoed), plus an
 * on-demand reveal of the pooled connection strings. Every mutation refetches
 * its list so the panel reflects the operator.
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
  const credentialsQuery = useQuery(DatabaseCredentialsDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });
  const [setAllowListMut, { loading: savingAllowList }] = useMutation(
    SetDatabaseIpAllowListDocument,
  );
  const [createCredentialMut, { loading: creatingCredential }] = useMutation(
    CreateDatabaseCredentialDocument,
  );
  const [deleteCredentialMut] = useMutation(DeleteDatabaseCredentialDocument);

  // Entries, not bare CIDRs: each carries the operator-facing description a
  // human gave it, which persists end to end (w4/m24).
  const allowList: IPAllowListEntryDraft[] = (
    allowListQuery.data?.database?.ipAllowListEntries ?? []
  )
    .filter((e): e is NonNullable<typeof e> => e != null)
    .map((e) => ({ cidrBlock: e.cidrBlock, description: e.description ?? "" }));

  const credentials = (credentialsQuery.data?.databaseCredentials ?? []).filter(
    (
      c,
    ): c is {
      __typename: "DatabaseCredential";
      username: string;
      default: boolean;
    } => !!c?.username,
  );

  // Each credential write answers the resulting list; store it as the query's
  // result instead of refetching.
  const storeCredentials = useCallback(
    (list: DatabaseCredentialsQuery["databaseCredentials"] | undefined) => {
      if (list)
        client.cache.writeQuery({
          query: DatabaseCredentialsDocument,
          variables: { id },
          data: { databaseCredentials: list },
        });
    },
    [client, id],
  );

  const saveAllowList = useCallback(
    async (entries: IPAllowListEntryDraft[]): Promise<boolean> => {
      try {
        await setAllowListMut({ variables: { id, entries } });
        toast.success(t("databases.accessAllowListSaved"));
        void allowListQuery.refetch();
        return true;
      } catch (e) {
        toast.error(
          mutationErrorMessage(e, t("databases.accessAllowListError")),
        );
        return false;
      }
    },
    [setAllowListMut, allowListQuery, id, t],
  );

  const createCredential = useCallback(
    async (name: string): Promise<boolean> => {
      try {
        const res = await createCredentialMut({
          variables: { id, username: name },
        });
        storeCredentials(res.data?.createDatabaseCredential);
        toast.success(t("databases.accessCredentialCreated", { name }));
        return true;
      } catch (e) {
        toast.error(
          hasGraphQLErrorCode(e, "POSTGRES_IDENTIFIER_RESERVED")
            ? t("databases.fieldPhysicalNameReserved", { name })
            : mutationErrorMessage(e, t("databases.accessCredentialError")),
        );
        return false;
      }
    },
    [createCredentialMut, storeCredentials, id, t],
  );

  const deleteCredential = useCallback(
    async (name: string): Promise<void> => {
      try {
        const res = await deleteCredentialMut({
          variables: { id, username: name },
        });
        storeCredentials(res.data?.deleteDatabaseCredential);
        toast.success(t("databases.accessCredentialDeleted", { name }));
      } catch (err) {
        toast.error(
          mutationErrorMessage(
            err,
            t("databases.accessCredentialDeleteError", { name }),
          ),
        );
      }
    },
    [deleteCredentialMut, storeCredentials, id, t],
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
    } catch (err) {
      toast.error(mutationErrorMessage(err, t("databases.accessPoolerError")));
    } finally {
      setPoolLoading(false);
    }
  }, [client, id, t]);

  return {
    allowList,
    isPublic: allowListQuery.data?.database?.public,
    credentials,
    loading: allowListQuery.loading || credentialsQuery.loading,
    savingAllowList,
    creatingCredential,
    saveAllowList,
    createCredential,
    deleteCredential,
    pooled,
    poolLoading,
    revealPooled,
  };
}
