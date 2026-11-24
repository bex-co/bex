import { useCallback, useState } from "react";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  BuildSourcesDocument,
  DeleteBuildSourceDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

export interface UseDeleteBuildSourceResult {
  remove: (id: string, name?: string) => Promise<boolean>;
  busy: boolean;
}

/** Deletes a Build Source, evicts it from Apollo cache, and refetches lists. */
export function useDeleteBuildSource(): UseDeleteBuildSourceResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const [mutate] = useMutation(DeleteBuildSourceDocument);
  const [busy, setBusy] = useState(false);

  const remove = useCallback(
    async (id: string, name?: string) => {
      setBusy(true);
      try {
        const res = await mutate({
          variables: { id },
          update: (cache) => {
            const cacheId = cache.identify({ __typename: "BuildSource", id });
            if (cacheId) {
              cache.evict({ id: cacheId });
              cache.gc();
            }
          },
        });
        if (!res.data?.deleteBuildSource) {
          throw new Error("deleteBuildSource returned false");
        }
        await client
          .refetchQueries({ include: [BuildSourcesDocument] })
          .catch(() => undefined);
        toast.success(
          t("buildSources.deleteSuccess", { name: name ?? id }),
        );
        return true;
      } catch (err) {
        toast.error(mutationErrorMessage(err, t("buildSources.deleteError")));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [client, mutate, t],
  );

  return { remove, busy };
}
