import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@apollo/client/react";
import {
  ServiceEventsDocument,
  type ServiceEventsQuery,
} from "@/graphql/definitions";

export type ServiceEventView = NonNullable<
  NonNullable<ServiceEventsQuery["serviceEvents"]>[number]
> & { id: string };

export interface UseServiceEventsResult {
  events: ServiceEventView[];
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
  loadMore: () => Promise<void>;
}

export interface UseServiceEventsOptions {
  limit: number;
  startTime: string;
  endTime: string;
  /**
   * Continue into fixed-width windows before startTime after cursor pages in
   * the current window are exhausted. The lower bound is normally the
   * service's creation time.
   */
  historyStartTime?: string;
  windowHours?: number;
  /** Fetch every cursor page in the selected window (used by chart markers). */
  autoPaginate?: boolean;
}

interface PageState {
  startTime: string;
  endTime: string;
  cursor?: string;
  exhausted: boolean;
}

const DEFAULT_WINDOW_HOURS = 720;

function eventViews(
  events: ServiceEventsQuery["serviceEvents"] | undefined,
): ServiceEventView[] {
  return (events ?? []).filter(
    (event): event is ServiceEventView => event != null && !!event.id,
  );
}

function mergeEvents(
  current: ServiceEventsQuery["serviceEvents"],
  next: ServiceEventsQuery["serviceEvents"],
): ServiceEventsQuery["serviceEvents"] {
  const seen = new Set<string>();
  return [...(current ?? []), ...(next ?? [])].filter((event) => {
    if (!event?.id || seen.has(event.id)) return false;
    seen.add(event.id);
    return true;
  });
}

function validTime(value: string | undefined): number | null {
  const parsed = Date.parse(value ?? "");
  return Number.isFinite(parsed) ? parsed : null;
}

/** Shared explicit-range service-event read for Events and Metrics. */
export function useServiceEvents(
  serviceId: string,
  options: UseServiceEventsOptions,
): UseServiceEventsResult {
  const {
    limit,
    startTime,
    endTime,
    historyStartTime,
    windowHours = DEFAULT_WINDOW_HOURS,
    autoPaginate = false,
  } = options;
  const variables = useMemo(
    () => ({ serviceId, startTime, endTime, limit }),
    [serviceId, startTime, endTime, limit],
  );
  const { data, loading, error, refetch, fetchMore } = useQuery(
    ServiceEventsDocument,
    {
      variables,
      fetchPolicy: "cache-and-network",
      notifyOnNetworkStatusChange: true,
    },
  );
  const pageRef = useRef<PageState | null>(null);
  const requestKey = `${serviceId}\u0000${startTime}\u0000${endTime}\u0000${limit}`;
  const requestKeyRef = useRef(requestKey);
  const historyStartRef = useRef(historyStartTime);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);

  historyStartRef.current = historyStartTime;

  if (requestKeyRef.current !== requestKey) {
    requestKeyRef.current = requestKey;
    pageRef.current = null;
  }

  const canEnterEarlierWindow = useCallback((page: PageState): boolean => {
    const floor = validTime(historyStartRef.current);
    const currentStart = validTime(page.startTime);
    return floor !== null && currentStart !== null && currentStart > floor;
  }, []);

  useEffect(() => {
    setLoadingMore(false);
    setHasMore(false);
  }, [requestKey]);

  useEffect(() => {
    if (!data || pageRef.current) return;
    const page = eventViews(data.serviceEvents);
    const state = {
      startTime,
      endTime,
      cursor: page.at(-1)?.cursor ?? undefined,
      exhausted: page.length < limit,
    };
    pageRef.current = state;
    setHasMore(!state.exhausted || canEnterEarlierWindow(state));
  }, [canEnterEarlierWindow, data, endTime, limit, startTime]);

  useEffect(() => {
    const page = pageRef.current;
    if (!page) return;
    setHasMore(!page.exhausted || canEnterEarlierWindow(page));
  }, [canEnterEarlierWindow, historyStartTime]);

  const loadMore = useCallback(async () => {
    if (loadingMore) return;
    let page = pageRef.current;
    if (!page) return;

    if (page.exhausted) {
      const floor = validTime(historyStartRef.current);
      const priorEnd = validTime(page.startTime);
      if (floor === null || priorEnd === null || priorEnd <= floor) {
        setHasMore(false);
        return;
      }
      const priorStart = Math.max(
        floor,
        priorEnd - windowHours * 60 * 60 * 1000,
      );
      page = {
        startTime: new Date(priorStart).toISOString(),
        endTime: new Date(priorEnd).toISOString(),
        exhausted: false,
      };
      pageRef.current = page;
    }

    setLoadingMore(true);
    try {
      const result = await fetchMore({
        variables: {
          serviceId,
          startTime: page.startTime,
          endTime: page.endTime,
          cursor: page.cursor,
          limit,
        },
        updateQuery(previous, { fetchMoreResult }) {
          return {
            ...previous,
            serviceEvents: mergeEvents(
              previous.serviceEvents,
              fetchMoreResult.serviceEvents,
            ),
          };
        },
      });
      const next = eventViews(result.data?.serviceEvents);
      const current = pageRef.current;
      if (!current) return;
      current.cursor = next.at(-1)?.cursor ?? current.cursor;
      current.exhausted = next.length < limit;
      setHasMore(!current.exhausted || canEnterEarlierWindow(current));
    } finally {
      setLoadingMore(false);
    }
  }, [
    canEnterEarlierWindow,
    fetchMore,
    limit,
    loadingMore,
    serviceId,
    windowHours,
  ]);

  useEffect(() => {
    if (!autoPaginate || loading || loadingMore || !hasMore) return;
    void loadMore();
  }, [autoPaginate, data, hasMore, loadMore, loading, loadingMore]);

  const resetAndRefetch = useCallback(async () => {
    pageRef.current = null;
    setHasMore(false);
    return refetch(variables);
  }, [refetch, variables]);

  const events = eventViews(data?.serviceEvents);
  return {
    events,
    loading,
    loadingMore,
    hasMore,
    error,
    refetch: resetAndRefetch,
    loadMore,
  };
}
