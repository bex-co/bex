import { useCallback, useEffect, useRef, useState } from "react";
import { useApolloClient } from "@apollo/client/react";
import {
  LogsDocument,
  type LogsQuery,
  type LogsQueryVariables,
} from "@/graphql/definitions";
import { dedupeLogLines, toLogLines } from "../lib/map";
import type { LogLine } from "../types";

type LogEnvelope = LogsQuery["logs"];

export interface OlderLogPages {
  /** Pages fetched behind the first page, oldest first. */
  older: LogLine[];
  /** True when the server says more history exists older than what's loaded. */
  hasMore: boolean;
  loadingOlder: boolean;
  /** Fetch the next older page and prepend it. No-op when !hasMore. */
  loadOlder: () => void;
}

/**
 * Walks one `logs(...)` query backwards through Render's paging envelope
 * (`hasMore` + `nextStartTime`/`nextEndTime`, w4/m107). The caller owns the
 * first page's query; this owns everything older than it, so every log surface
 * that reads `LogsDocument` pages the same way instead of stopping at the
 * 100-row cap (w4/m136).
 *
 * The cursor is seeded from `first` and then advanced by `loadOlder`, which is
 * why it is state rather than derived. A re-fetched first page (polling, a
 * cache update) re-seeds it only while nothing older is loaded — once the user
 * has paged back, the cursor already points further back than the first
 * page's. A `variables` change drops the older pages and discards any
 * in-flight response.
 */
export function useOlderLogPages(
  variables: LogsQueryVariables,
  first: LogEnvelope | undefined,
): OlderLogPages {
  const client = useApolloClient();
  const [older, setOlder] = useState<LogLine[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [cursor, setCursor] = useState<{
    startTime: string;
    endTime: string;
  } | null>(null);
  const [loadingOlder, setLoadingOlder] = useState(false);
  // Ignore a late older-page response after the query inputs change.
  const pageGen = useRef(0);
  const pagedBack = useRef(false);

  useEffect(() => {
    pageGen.current += 1;
    pagedBack.current = false;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- resetting paging state when the query inputs change; the bumped generation is what discards an in-flight older page
    setOlder((prev) => (prev.length === 0 ? prev : []));
    setLoadingOlder(false);
  }, [variables]);

  useEffect(() => {
    if (!first || pagedBack.current) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- seeding state that loadOlder then owns
    setHasMore(first.hasMore);
    // Keep the previous object when the cursor is unchanged, so an identical
    // re-fetched page (a poll) costs no re-render.
    setCursor((prev) =>
      prev?.startTime === first.nextStartTime &&
      prev.endTime === first.nextEndTime
        ? prev
        : { startTime: first.nextStartTime, endTime: first.nextEndTime },
    );
  }, [first]);

  const loadOlder = useCallback(() => {
    if (!hasMore || loadingOlder || !cursor) return;
    const gen = pageGen.current;
    setLoadingOlder(true);
    void client
      .query({
        query: LogsDocument,
        variables: {
          ...variables,
          startTime: cursor.startTime,
          endTime: cursor.endTime,
        },
        fetchPolicy: "network-only",
        errorPolicy: "all",
      })
      .then((result) => {
        if (gen !== pageGen.current) return;
        const env = result.data?.logs;
        if (!env) return;
        pagedBack.current = true;
        const page = toLogLines(env.logs);
        setOlder((prev) => dedupeLogLines([...page, ...prev]));
        setHasMore(env.hasMore);
        setCursor({ startTime: env.nextStartTime, endTime: env.nextEndTime });
      })
      .finally(() => {
        if (gen === pageGen.current) setLoadingOlder(false);
      });
  }, [hasMore, loadingOlder, cursor, client, variables]);

  return { older, hasMore, loadingOlder, loadOlder };
}
