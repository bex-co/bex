import { useCallback, useState } from "react";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetEnvironmentEnvGroupsDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

export interface UseSetEnvironmentEnvGroupsResult {
  setEnvGroups: (
    id: string,
    environmentName: string,
    envGroupIds: string[],
  ) => Promise<boolean>;
  /**
   * The environment being written, until its placement has been re-read —
   * after a failure too, so no second write starts from a stale view.
   */
  busyId: string | null;
}

const PLACEMENT_QUERIES = ["Environments", "EnvGroups"];

/** Full-replaces the environment groups scoped to one Environment. */
export function useSetEnvironmentEnvGroups(): UseSetEnvironmentEnvGroupsResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const [mutate] = useMutation(SetEnvironmentEnvGroupsDocument, {
    refetchQueries: PLACEMENT_QUERIES,
    awaitRefetchQueries: true,
  });
  const [busyId, setBusyId] = useState<string | null>(null);

  const setEnvGroups = useCallback(
    async (id: string, environmentName: string, envGroupIds: string[]) => {
      // A failed replacement is not known to have changed nothing: groups move
      // one at a time, and a request that timed out may still have been
      // applied (w4/m206). Read where the groups are now before the caller
      // can write again; a failed read offers the read again, never the write.
      const readPlacement = async () => {
        try {
          await client.refetchQueries({ include: PLACEMENT_QUERIES });
        } catch {
          toast.warning(
            t("environments.assignEnvGroupsPlacementUnread", {
              name: environmentName,
            }),
            {
              action: {
                label: t("common.tryAgain"),
                onClick: () => void readPlacement(),
              },
            },
          );
        }
      };
      setBusyId(id);
      try {
        await mutate({ variables: { id, envGroupIds } });
        toast.success(
          t("environments.assignEnvGroupsSuccess", {
            name: environmentName,
          }),
        );
        return true;
      } catch (err) {
        toast.error(
          mutationErrorMessage(
            err,
            t("environments.assignEnvGroupsError", { name: environmentName }),
          ),
        );
        await readPlacement();
        return false;
      } finally {
        setBusyId(null);
      }
    },
    [client, mutate, t],
  );

  return { setEnvGroups, busyId };
}
