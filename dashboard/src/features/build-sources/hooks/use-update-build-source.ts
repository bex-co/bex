import { useCallback, useState } from "react";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  BuildSourceDocument,
  BuildSourcesDocument,
  UpdateBuildSourceDocument,
  type UpdateBuildSourceMutationVariables,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  hasGraphQLErrorCode,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
import { BUILD_SOURCE_NAME_TAKEN_CODE } from "@/features/build-sources/lib/create-build-source-input";
import {
  mapBuildSource,
  type BuildSourceView,
} from "@/features/build-sources/types";

export interface UseUpdateBuildSourceResult {
  update: (
    variables: UpdateBuildSourceMutationVariables,
  ) => Promise<BuildSourceView | null>;
  busy: boolean;
  nameTaken: boolean;
  clearNameTaken: () => void;
}

/** Updates a Build Source's name or git/image settings and refreshes caches. */
export function useUpdateBuildSource(): UseUpdateBuildSourceResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const [mutate] = useMutation(UpdateBuildSourceDocument);
  const [busy, setBusy] = useState(false);
  const [nameTaken, setNameTaken] = useState(false);
  const clearNameTaken = useCallback(() => setNameTaken(false), []);

  const update = useCallback(
    async (variables: UpdateBuildSourceMutationVariables) => {
      setBusy(true);
      setNameTaken(false);
      try {
        const res = await mutate({ variables });
        const updated = mapBuildSource(res.data?.updateBuildSource);
        if (!updated) throw new Error("updateBuildSource returned no build source");
        await client
          .refetchQueries({
            include: [BuildSourcesDocument, BuildSourceDocument],
          })
          .catch(() => undefined);
        toast.success(t("buildSources.updateSuccess", { name: updated.name }));
        return updated;
      } catch (err) {
        if (hasGraphQLErrorCode(err, BUILD_SOURCE_NAME_TAKEN_CODE)) {
          setNameTaken(true);
          return null;
        }
        toast.error(mutationErrorMessage(err, t("buildSources.updateError")));
        return null;
      } finally {
        setBusy(false);
      }
    },
    [client, mutate, t],
  );

  return { update, busy, nameTaken, clearNameTaken };
}
