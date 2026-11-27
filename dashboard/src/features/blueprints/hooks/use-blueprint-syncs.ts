import { useMemo, useState } from "react";
import { useApolloClient, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import { BlueprintSyncsDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { skipPollWhenHidden, useConvergingPoll } from "@/common/lib/polling";
import {
  NO_APPENDED,
  appendPage,
  joinAppendedPages,
  type AppendedPages,
} from "@/common/lib/appended-pages";
import type { BlueprintSyncView } from "@/features/blueprints/types";
import { toBlueprintSyncView } from "@/features/blueprints/lib/views";
import { isApplyingSyncState } from "@/features/blueprints/lib/sync-state";
import { nonNull } from "@/common/lib/non-null";

/** Render reads a Blueprint's syncs 20 at a time. */
export const BLUEPRINT_SYNCS_PAGE_SIZE = 20;

export interface UseBlueprintSyncsResult {
  syncs: BlueprintSyncView[];
  loading: boolean;
  loadingMore: boolean;
  error: Error | undefined;
  hasMore: boolean;
  loadMore: () => void;
}

function toRows(
  raw: ReadonlyArray<Parameters<typeof toBlueprintSyncView>[0] | null> | null,
) {
  return (raw ?? []).filter(nonNull).map(toBlueprintSyncView);
}

/**
 * A Blueprint's syncs, newest first, for Syncs (w3/m678): the first page
 * polled, so a sync a push, the CLI or Manual sync starts appears at the top
 * without a reload (Render prepends each new sync through a subscription),
 * fast while one applies; older pages read with Load more past the
 * last row shown and appended (common/lib/appended-pages), as the deploy
 * history's are.
 */
export function useBlueprintSyncs(
  blueprintId: string,
): UseBlueprintSyncsResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const { data, loading, error, startPolling, stopPolling } = useQuery(
    BlueprintSyncsDocument,
    {
      variables: { id: blueprintId, limit: BLUEPRINT_SYNCS_PAGE_SIZE },
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      skipPollAttempt: skipPollWhenHidden,
    },
  );
  const firstPage = useMemo(() => toRows(data?.blueprintSyncs ?? null), [data]);
  const [appended, setAppended] =
    useState<AppendedPages<BlueprintSyncView>>(NO_APPENDED);
  const [loadingMore, setLoadingMore] = useState(false);
  const {
    rows: syncs,
    hasMore,
    next,
  } = useMemo(
    () =>
      joinAppendedPages(
        firstPage,
        appended,
        blueprintId,
        BLUEPRINT_SYNCS_PAGE_SIZE,
      ),
    [appended, blueprintId, firstPage],
  );
  useConvergingPoll(
    startPolling,
    stopPolling,
    firstPage.some((s) => isApplyingSyncState(s.state)),
  );

  async function loadMore() {
    if (loadingMore || !next) return;
    const request = next;
    setLoadingMore(true);
    try {
      const result = await client.query({
        query: BlueprintSyncsDocument,
        variables: {
          id: blueprintId,
          cursor: request.cursor,
          limit: BLUEPRINT_SYNCS_PAGE_SIZE,
        },
        fetchPolicy: "network-only",
        errorPolicy: "all",
      });
      if (result.data?.blueprintSyncs) {
        const page = toRows(result.data.blueprintSyncs);
        setAppended((prev) => appendPage(prev, request, page));
      }
      if (result.error || !result.data?.blueprintSyncs) throw result.error;
    } catch {
      toast.error(t("blueprints.syncsLoadMoreError"));
    } finally {
      setLoadingMore(false);
    }
  }

  return {
    syncs,
    loading: loading && syncs.length === 0,
    loadingMore,
    error,
    hasMore,
    loadMore: () => void loadMore(),
  };
}
