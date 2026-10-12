import type { ApolloClient } from "@apollo/client";
import { toast } from "sonner";
import { EnvironmentsDocument } from "@/graphql/definitions";

/**
 * Re-read a project's environments after a write so the list shows it — a
 * create (w4/231) or a membership change (w4/263).
 *
 * A named refetchQueries dedupes onto an Environments read already in flight,
 * one that may have started before the write and so answers with the
 * pre-write list. So: first let every Environments read that is IN FLIGHT settle
 * (joining it issues no new request; each result is settled on its own, since
 * the aggregate promise rejects on the first failure while others may still be
 * writing), so none can land later with the pre-save list. Only then read this
 * project's list network-only, with queryDeduplication off so it can't join an
 * older read. Its answer is authoritative and reaches every useEnvironments
 * consumer through the cache; its failure touches no watcher, so the rows on
 * screen stay (the use-rename-project precedent, w4/m186).
 */
export async function refreshEnvironments(
  client: ApolloClient,
  projectId: string,
) {
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
 * After a committed write, refresh the project's environments and toast the
 * outcome. The write already happened: a failed refresh is never a failed
 * write — it warns, and its retry re-reads without writing again.
 */
export function publishEnvironments(
  client: ApolloClient,
  projectId: string,
  messages: { succeeded: string; refreshFailed: string; tryAgain: string },
): Promise<void> {
  const attempt = async () => {
    try {
      await refreshEnvironments(client, projectId);
    } catch {
      toast.warning(messages.refreshFailed, {
        action: { label: messages.tryAgain, onClick: () => void attempt() },
      });
      return;
    }
    toast.success(messages.succeeded);
  };
  return attempt();
}
