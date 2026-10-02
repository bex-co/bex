import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@apollo/client/react";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
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
  /** Refresh the bounded current head, retaining loaded historical pages. */
  live?: boolean;
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
  const byId = new Map<string, ServiceEventView>();
  for (const event of eventViews(current)) byId.set(event.id, event);
  for (const event of eventViews(next)) byId.set(event.id, event);
  return [...byId.values()].sort(
    (a, b) =>
      (validTime(b.timestamp ?? undefined) ?? 0) -
      (validTime(a.timestamp ?? undefined) ?? 0),
  );
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
    live = false,
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
      errorPolicy: "all",
    },
  );
  const pageRef = useRef<PageState | null>(null);
  const requestKey = `${live}\u0000${serviceId}\u0000${startTime}\u0000${endTime}\u0000${limit}`;
  const requestKeyRef = useRef(requestKey);
  const generationRef = useRef(0);
  const historyStartRef = useRef(historyStartTime);
  const mountedRef = useRef(true);
  const loadingMoreRef = useRef(false);
  const refreshingRef = useRef(false);
  const eventsRef = useRef(data?.serviceEvents);
  const [readError, setReadError] = useState<Error>();
  eventsRef.current = data?.serviceEvents;

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);

  historyStartRef.current = historyStartTime;

  if (requestKeyRef.current !== requestKey) {
    requestKeyRef.current = requestKey;
    generationRef.current++;
    pageRef.current = null;
    loadingMoreRef.current = false;
    refreshingRef.current = false;
  }

  const canEnterEarlierWindow = useCallback((page: PageState): boolean => {
    const floor = validTime(historyStartRef.current);
    const currentStart = validTime(page.startTime);
    return floor !== null && currentStart !== null && currentStart > floor;
  }, []);

  useEffect(() => {
    setLoadingMore(false);
    setHasMore(false);
    setReadError(undefined);
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
    if (loadingMoreRef.current) return;
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

    const generation = generationRef.current;
    const isCurrent = () =>
      mountedRef.current &&
      requestKeyRef.current === requestKey &&
      generationRef.current === generation;
    loadingMoreRef.current = true;
    setLoadingMore(true);
    setReadError(undefined);
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
          if (!isCurrent()) return previous;
          return {
            ...previous,
            serviceEvents: mergeEvents(
              previous.serviceEvents,
              fetchMoreResult.serviceEvents,
            ),
          };
        },
      });
      if (!isCurrent()) return;
      if (result.error) throw result.error;
      const next = eventViews(result.data?.serviceEvents);
      const current = pageRef.current;
      if (!current) return;
      current.cursor = next.at(-1)?.cursor ?? current.cursor;
      current.exhausted = next.length < limit;
      setHasMore(!current.exhausted || canEnterEarlierWindow(current));
    } catch (cause) {
      if (isCurrent())
        setReadError(cause instanceof Error ? cause : new Error(String(cause)));
    } finally {
      if (isCurrent()) {
        loadingMoreRef.current = false;
        setLoadingMore(false);
      }
    }
  }, [
    canEnterEarlierWindow,
    fetchMore,
    limit,
    requestKey,
    serviceId,
    windowHours,
  ]);

  useEffect(() => {
    if (!autoPaginate || loading || loadingMore || !hasMore || readError)
      return;
    void loadMore();
  }, [autoPaginate, data, hasMore, loadMore, loading, loadingMore, readError]);

  const refreshHead = useCallback(async () => {
    if (refreshingRef.current) return;
    const generation = generationRef.current;
    const isCurrent = () =>
      mountedRef.current &&
      requestKeyRef.current === requestKey &&
      generationRef.current === generation;
    refreshingRef.current = true;
    setReadError(undefined);
    const now = Date.now();
    const headStart =
      now - Math.min(windowHours, DEFAULT_WINDOW_HOURS) * 60 * 60 * 1000;
    // Scan through the loaded portion, not merely to the first overlapping ID:
    // a fact can arrive late with an older occurrence time.
    const oldest = eventViews(eventsRef.current).at(-1)?.timestamp;
    const through = Math.max(
      headStart,
      validTime(oldest ?? undefined) ?? headStart,
    );
    let cursor: string | undefined;
    try {
      do {
        const result = await fetchMore({
          variables: {
            serviceId,
            startTime: new Date(headStart).toISOString(),
            endTime: new Date(now).toISOString(),
            limit,
            cursor,
          },
          updateQuery(previous, { fetchMoreResult }) {
            if (!isCurrent()) return previous;
            return {
              ...previous,
              serviceEvents: mergeEvents(
                previous?.serviceEvents,
                fetchMoreResult.serviceEvents,
              ),
            };
          },
        });
        if (!isCurrent()) return;
        if (result.error) throw result.error;
        const page = eventViews(result.data?.serviceEvents);
        const last = page.at(-1);
        const nextCursor = last?.cursor ?? undefined;
        if (
          page.length < limit ||
          (validTime(last?.timestamp ?? undefined) ?? 0) < through ||
          !nextCursor ||
          nextCursor === cursor
        )
          break;
        cursor = nextCursor;
      } while (isCurrent());
    } catch (cause) {
      if (isCurrent())
        setReadError(cause instanceof Error ? cause : new Error(String(cause)));
    } finally {
      if (isCurrent()) refreshingRef.current = false;
    }
  }, [fetchMore, limit, requestKey, serviceId, windowHours]);

  useEffect(() => {
    if (!live) return;
    const timer = setInterval(() => {
      if (!skipPollWhenHidden()) void refreshHead();
    }, RESOURCE_POLL_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [live, refreshHead]);

  const resetAndRefetch = useCallback(async () => {
    setReadError(undefined);
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
    error: readError ?? error,
    refetch: live ? refreshHead : resetAndRefetch,
    loadMore,
  };
}
