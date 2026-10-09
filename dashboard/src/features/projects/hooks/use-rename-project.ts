import { useCallback, useState } from "react";
import type { ApolloClient } from "@apollo/client";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { useRouter } from "@tanstack/react-router";
import { toast } from "sonner";
import {
  ProjectDocument,
  ProjectsDocument,
  RenameProjectDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

export interface UseRenameProjectResult {
  /**
   * Fires renameProject exactly once. Resolves true when the server accepted
   * the new name — even if the follow-up refresh failed, which is reported
   * separately and never retries the write — and false when it refused.
   */
  rename: (id: string, name: string) => Promise<boolean>;
  /** True from submit until the saved name is published (or its refresh failed). */
  busy: boolean;
}

/**
 * Re-read a renamed project so the page publishes the saved name (w4/m186).
 *
 * The project page's heading and title come from its route loader, which
 * router.invalidate() re-runs cache-first for a retained match. A Projects
 * list read that started before the mutation can finish after it and write
 * the old name back over the mutation's normalized Project. So: settle every
 * active Projects read first — refetchQueries dedupes onto an in-flight read,
 * and each result is settled on its own because the aggregate promise rejects
 * on the first failure while others may still be writing — and only then
 * re-read the project by id. queryDeduplication:false keeps that last read
 * from joining an older in-flight one. Whatever it returns is authoritative,
 * including a newer name another actor saved in the meantime.
 */
async function refreshRenamedProject(client: ApolloClient, id: string) {
  const lists = client.refetchQueries({ include: [ProjectsDocument] });
  lists.catch(() => undefined); // each result is settled below
  await Promise.allSettled(lists.results);
  await client.query({
    query: ProjectDocument,
    variables: { id },
    fetchPolicy: "network-only",
    context: { queryDeduplication: false },
  });
}

/** Wires both project rename entry points (Overview, Settings) to bex-api's `renameProject`. */
export function useRenameProject(): UseRenameProjectResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const router = useRouter();
  const [mutate] = useMutation(RenameProjectDocument);
  const [busy, setBusy] = useState(false);

  // The write already committed: a failed refresh is never a failed rename,
  // and its retry re-reads without renaming again.
  const publish = useCallback(
    (id: string, name: string): Promise<void> => {
      const attempt = async () => {
        try {
          await refreshRenamedProject(client, id);
        } catch {
          toast.warning(t("projects.renameRefreshFailed", { name }), {
            action: {
              label: t("common.tryAgain"),
              onClick: () => void attempt(),
            },
          });
          return;
        }
        await router.invalidate().catch(() => undefined);
        toast.success(t("projects.renameSuccess", { name }));
      };
      return attempt();
    },
    [client, router, t],
  );

  const rename = useCallback(
    async (id: string, name: string) => {
      setBusy(true);
      try {
        try {
          await mutate({ variables: { id, name } });
        } catch (err) {
          toast.error(
            mutationErrorMessage(err, t("projects.renameError", { name })),
          );
          return false;
        }
        await publish(id, name);
        return true;
      } finally {
        setBusy(false);
      }
    },
    [mutate, publish, t],
  );

  return { rename, busy };
}
