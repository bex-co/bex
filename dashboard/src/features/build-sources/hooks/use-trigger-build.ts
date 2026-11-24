import { useCallback, useState } from "react";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  BuildSourceBuildsDocument,
  TriggerBuildDocument,
  type TriggerBuildMutationVariables,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import {
  mapBuildSourceBuild,
  type BuildSourceBuildView,
} from "@/features/build-sources/types";

export interface UseTriggerBuildResult {
  trigger: (
    variables: TriggerBuildMutationVariables,
  ) => Promise<BuildSourceBuildView | null>;
  busy: boolean;
}

/** Triggers a new build for a git-backed Build Source and refreshes its build list. */
export function useTriggerBuild(): UseTriggerBuildResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const [mutate] = useMutation(TriggerBuildDocument);
  const [busy, setBusy] = useState(false);

  const trigger = useCallback(
    async (variables: TriggerBuildMutationVariables) => {
      setBusy(true);
      try {
        const res = await mutate({ variables });
        const build = mapBuildSourceBuild(
          res.data?.triggerBuild,
          variables.buildSourceId,
        );
        if (!build) throw new Error("triggerBuild returned no build");
        await client
          .refetchQueries({ include: [BuildSourceBuildsDocument] })
          .catch(() => undefined);
        toast.success(t("buildSources.triggerBuildSuccess"));
        return build;
      } catch (err) {
        toast.error(
          mutationErrorMessage(err, t("buildSources.triggerBuildError")),
        );
        return null;
      } finally {
        setBusy(false);
      }
    },
    [client, mutate, t],
  );

  return { trigger, busy };
}
