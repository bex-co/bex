import { useCallback, useState } from "react";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  BuildSourcesDocument,
  CreateBuildSourceDocument,
  ServicesDocument,
  type CreateBuildSourceMutationVariables,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  hasGraphQLErrorCode,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  BUILD_SOURCE_NAME_TAKEN_CODE,
  extractPartiallyLinkedError,
} from "@/features/build-sources/lib/create-build-source-input";
import {
  mapBuildSource,
  type BuildSourcePartiallyLinked,
  type BuildSourceView,
} from "@/features/build-sources/types";

export type CreateBuildSourceParams = Omit<
  CreateBuildSourceMutationVariables,
  "ownerId"
>;

export interface UseCreateBuildSourceResult {
  create: (params: CreateBuildSourceParams) => Promise<BuildSourceView | null>;
  busy: boolean;
  /** True when the last submit was refused with `BUILD_SOURCE_NAME_TAKEN`. */
  nameTaken: boolean;
  clearNameTaken: () => void;
  /** Non-null when `409 BUILD_SOURCE_PARTIALLY_LINKED` created the source and linked some services before failing. */
  partiallyLinked: BuildSourcePartiallyLinked | null;
  clearPartiallyLinked: () => void;
}

/**
 * Creates a Build Source in the active workspace and refreshes every Build
 * Sources (and linked Services) query cache on both full success and
 * `409 BUILD_SOURCE_PARTIALLY_LINKED` (since the source exists on the server).
 */
export function useCreateBuildSource(): UseCreateBuildSourceResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const client = useApolloClient();
  const [mutate] = useMutation(CreateBuildSourceDocument);
  const [busy, setBusy] = useState(false);
  const [nameTaken, setNameTaken] = useState(false);
  const [partiallyLinked, setPartiallyLinked] =
    useState<BuildSourcePartiallyLinked | null>(null);

  const clearNameTaken = useCallback(() => setNameTaken(false), []);
  const clearPartiallyLinked = useCallback(() => setPartiallyLinked(null), []);

  const refreshCaches = useCallback(
    async (linkedServices: boolean) => {
      const include = linkedServices
        ? [BuildSourcesDocument, ServicesDocument]
        : [BuildSourcesDocument];
      await client.refetchQueries({ include }).catch(() => undefined);
    },
    [client],
  );

  const create = useCallback(
    async (params: CreateBuildSourceParams) => {
      if (currentWorkspaceId == null) {
        toast.error(t("buildSources.createError"));
        return null;
      }
      setBusy(true);
      setNameTaken(false);
      setPartiallyLinked(null);
      const hasServices = (params.serviceIds?.length ?? 0) > 0;
      try {
        const res = await mutate({
          variables: {
            ...params,
            ownerId: currentWorkspaceId,
          },
        });
        const created = mapBuildSource(res.data?.createBuildSource);
        if (!created) throw new Error("createBuildSource returned no build source");
        await refreshCaches(hasServices);
        toast.success(
          t("buildSources.createSuccess", { name: created.name }),
        );
        return created;
      } catch (err) {
        if (hasGraphQLErrorCode(err, BUILD_SOURCE_NAME_TAKEN_CODE)) {
          setNameTaken(true);
          return null;
        }
        const partial = extractPartiallyLinkedError(err);
        if (partial) {
          setPartiallyLinked(partial);
          // The Build Source and its first linked service(s) persisted on the
          // server; refresh every cache so the dashboard reflects reality.
          await refreshCaches(true);
          return null;
        }
        toast.error(mutationErrorMessage(err, t("buildSources.createError")));
        return null;
      } finally {
        setBusy(false);
      }
    },
    [currentWorkspaceId, mutate, refreshCaches, t],
  );

  return {
    create,
    busy,
    nameTaken,
    clearNameTaken,
    partiallyLinked,
    clearPartiallyLinked,
  };
}
