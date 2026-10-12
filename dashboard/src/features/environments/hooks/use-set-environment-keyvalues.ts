import { useCallback, useState } from "react";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetEnvironmentKeyValuesDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { publishEnvironments } from "@/features/environments/lib/refresh-environments";

export interface UseSetEnvironmentKeyValuesResult {
  /**
   * Full-replace an environment's key-value membership (bex-api's
   * `setEnvironmentKeyValues`, w6/m20 extension). Assigning key-value
   * instances auto-joins them to the environment's parent project, mirroring
   * useSetEnvironmentDatabases. Resolves true when the server accepted the
   * write — even if the follow-up list refresh failed, which is reported
   * separately and never repeats the write.
   */
  setKeyValues: (
    id: string,
    envName: string,
    keyValueIds: string[],
  ) => Promise<boolean>;
  /** The environment id currently being written, or null. */
  busyId: string | null;
}

/**
 * Wires the environment "Manage resources" dialog's Key Value tab and the bulk
 * Move to bex-api's `setEnvironmentKeyValues` — a full-replace verb.
 *
 * A move changes two environments' membership, so the project's list must be
 * re-read after the write: a named refetch could dedupe onto an older
 * Environments read that then lands with the source still holding the
 * instance (w4/263). `refreshEnvironments` settles those first, then reads.
 */
export function useSetEnvironmentKeyValues(): UseSetEnvironmentKeyValuesResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const [mutate] = useMutation(SetEnvironmentKeyValuesDocument, {
    refetchQueries: ["Projects"],
  });
  const [busyId, setBusyId] = useState<string | null>(null);

  const setKeyValues = useCallback(
    async (id: string, envName: string, keyValueIds: string[]) => {
      setBusyId(id);
      try {
        let projectId: string | null | undefined;
        try {
          const res = await mutate({ variables: { id, keyValueIds } });
          projectId = res.data?.setEnvironmentKeyValues?.projectId;
        } catch (err) {
          toast.error(
            mutationErrorMessage(
              err,
              t("environments.assignKeyValuesError", { name: envName }),
            ),
          );
          return false;
        }
        const succeeded = t("environments.assignKeyValuesSuccess", {
          name: envName,
        });
        if (projectId) {
          await publishEnvironments(client, projectId, {
            succeeded,
            refreshFailed: t("environments.assignKeyValuesRefreshFailed", {
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

  return { setKeyValues, busyId };
}
