import { useCallback, useState } from "react";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DeleteEnvironmentDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { publishEnvironments } from "@/features/environments/lib/refresh-environments";

export interface UseDeleteEnvironmentResult {
  /**
   * Fires deleteEnvironment exactly once. Resolves true when the server
   * deleted it — even if the follow-up list refresh failed, which is reported
   * separately and never deletes again — and false when it refused.
   */
  remove: (id: string, name: string) => Promise<boolean>;
  /**
   * The id being deleted, until the list no longer shows it (or its refresh
   * failed); null otherwise. Disables that environment's control.
   */
  deleting: string | null;
}

/**
 * Wires an environment's Delete action to bex-api's `deleteEnvironment` — its
 * services keep their project membership, they just lose the environment label
 * (docs/ADR032-environments.md).
 *
 * The list is re-read through `publishEnvironments`, not a named refetch: that
 * dedupes onto an Environments read already in flight, which answers with the
 * pre-delete list and leaves the deleted environment selected (w4/290).
 */
export function useDeleteEnvironment(
  projectId: string,
): UseDeleteEnvironmentResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const [mutate] = useMutation(DeleteEnvironmentDocument);
  const [deleting, setDeleting] = useState<string | null>(null);

  const remove = useCallback(
    async (id: string, name: string) => {
      setDeleting(id);
      try {
        try {
          await mutate({ variables: { id } });
        } catch (err) {
          toast.error(
            mutationErrorMessage(err, t("environments.deleteError", { name })),
          );
          return false;
        }
        await publishEnvironments(client, projectId, {
          succeeded: t("environments.deleteSuccess", { name }),
          refreshFailed: t("environments.deleteRefreshFailed", { name }),
          tryAgain: t("common.tryAgain"),
        });
        return true;
      } finally {
        setDeleting(null);
      }
    },
    [client, mutate, projectId, t],
  );

  return { remove, deleting };
}
