import { useCallback, useState } from "react";
import type { ApolloClient } from "@apollo/client";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CreateEnvironmentDocument,
  EnvironmentsDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

export interface UseCreateEnvironmentResult {
  /**
   * Fires createEnvironment exactly once. Resolves the new id when the server
   * created it — even if the follow-up list refresh failed, which is reported
   * separately and never retries the write — and null when it refused.
   */
  create: (name: string) => Promise<string | null>;
  /** True from submit until the new environment is listed (or its refresh failed). */
  busy: boolean;
}

/**
 * Re-read the project's environments so the created one is listed (w4/231).
 *
 * A named refetchQueries dedupes onto an Environments read already in flight,
 * one that may have started before the write and so answers without the new
 * environment. So: first let every Environments read that is IN FLIGHT settle
 * (joining it issues no new request; each result is settled on its own, since
 * the aggregate promise rejects on the first failure while others may still be
 * writing), so none can land later with the pre-save list. Only then read this
 * project's list network-only, with queryDeduplication off so it can't join an
 * older read. Its answer is authoritative and reaches every useEnvironments
 * consumer through the cache; its failure touches no watcher, so the rows on
 * screen stay (the use-rename-project precedent, w4/m186).
 */
async function refreshEnvironments(client: ApolloClient, projectId: string) {
  const lists = client.refetchQueries({
    include: [EnvironmentsDocument],
    onQueryUpdated: (query) =>
      query.getCurrentResult().loading ? query.refetch() : false,
  });
  lists.catch(() => undefined); // each result is settled below
  await Promise.allSettled(lists.results);
  await client.query({
    query: EnvironmentsDocument,
    variables: { projectId },
    fetchPolicy: "network-only",
    context: { queryDeduplication: false },
  });
}

/**
 * Wires the "New Environment" dialog to bex-api's `createEnvironment` (w1/m32,
 * bex extension). Scoped to a project — the workspace is inherited from the
 * project server-side, so the caller only supplies the name.
 */
export function useCreateEnvironment(
  projectId: string,
): UseCreateEnvironmentResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const [mutate] = useMutation(CreateEnvironmentDocument);
  const [busy, setBusy] = useState(false);

  // The write already committed: a failed refresh is never a failed create,
  // and its retry re-reads without creating again.
  const publish = useCallback(
    (name: string): Promise<void> => {
      const attempt = async () => {
        try {
          await refreshEnvironments(client, projectId);
        } catch {
          toast.warning(t("environments.createRefreshFailed", { name }), {
            action: {
              label: t("common.tryAgain"),
              onClick: () => void attempt(),
            },
          });
          return;
        }
        toast.success(t("environments.createSuccess", { name }));
      };
      return attempt();
    },
    [client, projectId, t],
  );

  const create = useCallback(
    async (name: string) => {
      setBusy(true);
      try {
        let id: string | null | undefined;
        try {
          const res = await mutate({ variables: { name, projectId } });
          id = res.data?.createEnvironment?.id;
          if (!id) throw new Error("createEnvironment returned no id");
        } catch (err) {
          toast.error(
            mutationErrorMessage(err, t("environments.createError", { name })),
          );
          return null;
        }
        await publish(name);
        return id;
      } finally {
        setBusy(false);
      }
    },
    [mutate, publish, t, projectId],
  );

  return { create, busy };
}
