import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
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
  busyId: string | null;
}

/** Full-replaces the environment groups scoped to one Environment. */
export function useSetEnvironmentEnvGroups(): UseSetEnvironmentEnvGroupsResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetEnvironmentEnvGroupsDocument, {
    refetchQueries: ["Environments", "EnvGroups"],
    awaitRefetchQueries: true,
  });
  const [busyId, setBusyId] = useState<string | null>(null);

  const setEnvGroups = useCallback(
    async (id: string, environmentName: string, envGroupIds: string[]) => {
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
        return false;
      } finally {
        setBusyId(null);
      }
    },
    [mutate, t],
  );

  return { setEnvGroups, busyId };
}
