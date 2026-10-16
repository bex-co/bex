import { useCallback, useState } from "react";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetEnvironmentDatabasesDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { publishEnvironments } from "@/features/environments/lib/refresh-environments";

export interface UseSetEnvironmentDatabasesResult {
  /**
   * Full-replace an environment's database membership (bex-api's
   * `setEnvironmentDatabases`, w6/m20 extension). Assigning databases
   * auto-joins them to the environment's parent project, mirroring
   * useSetEnvironmentServices. Resolves true when the server accepted the
   * write — even if the follow-up list refresh failed, which is reported
   * separately and never repeats the write.
   */
  setDatabases: (
    id: string,
    envName: string,
    databaseIds: string[],
  ) => Promise<boolean>;
  /** The environment id currently being written, or null. */
  busyId: string | null;
}

/**
 * Wires the environment "Manage resources" dialog's Databases tab and the bulk
 * Move to bex-api's `setEnvironmentDatabases` — a full-replace verb.
 *
 * A move changes two environments' membership, so the project's list must be
 * re-read after the write: a named refetch could dedupe onto an older
 * Environments read that then lands with the source still holding the
 * database (w4/325, as w4/263 found for Key Value). `publishEnvironments`
 * settles those first, then reads.
 */
export function useSetEnvironmentDatabases(): UseSetEnvironmentDatabasesResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const [mutate] = useMutation(SetEnvironmentDatabasesDocument, {
    refetchQueries: ["Projects"],
  });
  const [busyId, setBusyId] = useState<string | null>(null);

  const setDatabases = useCallback(
    async (id: string, envName: string, databaseIds: string[]) => {
      setBusyId(id);
      try {
        let projectId: string | null | undefined;
        try {
          const res = await mutate({ variables: { id, databaseIds } });
          projectId = res.data?.setEnvironmentDatabases?.projectId;
        } catch (err) {
          toast.error(
            mutationErrorMessage(
              err,
              t("environments.assignDatabasesError", { name: envName }),
            ),
          );
          return false;
        }
        const succeeded = t("environments.assignDatabasesSuccess", {
          name: envName,
        });
        if (projectId) {
          await publishEnvironments(client, projectId, {
            succeeded,
            refreshFailed: t("environments.assignDatabasesRefreshFailed", {
              name: envName,
            }),
            tryAgain: t("common.tryAgain"),
          });
        } else {
          toast.success(succeeded);
        }
        return true;
      } finally {
        setBusyId(null);
      }
    },
    [client, mutate, t],
  );

  return { setDatabases, busyId };
}
