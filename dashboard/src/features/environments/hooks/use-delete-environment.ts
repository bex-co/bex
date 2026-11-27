import { useCallback, useState } from "react";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { DeleteEnvironmentDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { relayableReason } from "@/common/lib/graphql-error";
import {
  DELETED_RESOURCE_QUERIES,
  publishEnvironments,
} from "@/features/environments/lib/refresh-environments";

export type DeleteEnvironmentResult =
  | { ok: true }
  /** `message` is the server's reason, or "" when it named none. */
  | { ok: false; message: string };

export interface UseDeleteEnvironmentResult {
  /**
   * Fires deleteEnvironment exactly once. Resolves ok when the server deleted
   * it — even if the follow-up list refresh failed, which is reported
   * separately and never deletes again — and with the refusal otherwise, for
   * the confirm dialog to show, as Render's does (w1/m683).
   */
  remove: (id: string, name: string) => Promise<DeleteEnvironmentResult>;
  /**
   * The id being deleted, until the list no longer shows it (or its refresh
   * failed); null otherwise. Disables that environment's control.
   */
  deleting: string | null;
}

/**
 * Wires an environment's Delete action to bex-api's `deleteEnvironment` — its
 * services, databases and key value stores are deleted with it; env groups just
 * lose the environment label (docs/ADR032-environments.md, w6/m290).
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
    async (id: string, name: string): Promise<DeleteEnvironmentResult> => {
      setDeleting(id);
      try {
        try {
          await mutate({ variables: { id } });
        } catch (err) {
          return { ok: false, message: relayableReason(err) };
        }
        client
          .refetchQueries({ include: DELETED_RESOURCE_QUERIES })
          .catch(() => undefined);
        await publishEnvironments(client, projectId, {
          succeeded: t("environments.deleteSuccess", { name }),
          refreshFailed: t("environments.deleteRefreshFailed", { name }),
          tryAgain: t("common.tryAgain"),
        });
        return { ok: true };
      } finally {
        setDeleting(null);
      }
    },
    [client, mutate, projectId, t],
  );

  return { remove, deleting };
}
