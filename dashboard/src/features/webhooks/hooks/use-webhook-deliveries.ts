import { useCallback, useEffect, useMemo, useState } from "react";
import { useApolloClient, useQuery } from "@apollo/client/react";
import { type WebhookDeliveriesQuery } from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { WebhookDeliveriesDocument } from "@/features/webhooks/api/operations";
import type {
  WebhookDeliveryView,
  WebhookDeliveryStatus,
} from "@/features/webhooks/types";
import { useWorkspace } from "@/features/workspaces/context/hooks";

const PAGE_SIZE = 20;

export interface WebhookDeliveryFilter {
  status?: "delivered" | "failed";
  sentAfter?: string;
  sentBefore?: string;
}

type RawDelivery = NonNullable<
  WebhookDeliveriesQuery["webhookDeliveries"]
>[number];

function toViews(
  raw: WebhookDeliveriesQuery["webhookDeliveries"] | undefined,
): WebhookDeliveryView[] {
  return (raw ?? [])
    .filter((d): d is NonNullable<RawDelivery> & { id: string } => !!d?.id)
    .map((d) => ({
      id: d.id,
      eventId: d.eventId ?? "",
      eventType: d.eventType ?? "",
      serviceId: d.serviceId ?? "",
      status: (d.status ?? "pending") as WebhookDeliveryStatus,
      attemptNumber: d.attemptNumber ?? 0,
      statusCode: d.statusCode ?? 0,
      transportError: d.transportError ?? "",
      responseBody: d.responseBody ?? "",
      requestBody: d.requestBody ?? "",
      sentAt: d.sentAt,
      nextAttemptAt: d.nextAttemptAt,
      parentStatus: (d.parentStatus ?? "pending") as WebhookDeliveryStatus,
      cursor: d.cursor ?? "",
    }));
}

function mergeAttempts(
  newest: WebhookDeliveryView[],
  retained: WebhookDeliveryView[],
): WebhookDeliveryView[] {
  const retainedById = new Map(
    retained.map((attempt) => [attempt.id, attempt]),
  );
  const seen = new Set<string>();
  const merged: WebhookDeliveryView[] = [];
  const append = (attempt: WebhookDeliveryView) => {
    if (seen.has(attempt.id)) return;
    seen.add(attempt.id);
    const previous = retainedById.get(attempt.id);
    merged.push(
      previous && sameAttempt(previous, attempt) ? previous : attempt,
    );
  };
  newest.forEach(append);
  retained.forEach(append);
  return merged;
}

function sameAttempt(
  left: WebhookDeliveryView,
  right: WebhookDeliveryView,
): boolean {
  return (
    left.id === right.id &&
    left.eventId === right.eventId &&
    left.eventType === right.eventType &&
    left.serviceId === right.serviceId &&
    left.status === right.status &&
    left.attemptNumber === right.attemptNumber &&
    left.statusCode === right.statusCode &&
    left.transportError === right.transportError &&
    left.responseBody === right.responseBody &&
    left.requestBody === right.requestBody &&
    left.sentAt === right.sentAt &&
    left.nextAttemptAt === right.nextAttemptAt &&
    left.parentStatus === right.parentStatus &&
    left.cursor === right.cursor
  );
}

export interface UseWebhookDeliveriesResult {
  deliveries: WebhookDeliveryView[];
  loading: boolean;
  loadingMore: boolean;
  error: Error | undefined;
  hasMore: boolean;
  loadMore: () => Promise<void>;
  refresh: () => Promise<unknown>;
}

/**
 * One endpoint's immutable attempt history, newest first and keyset-paged. The
 * cursor-less page polls while visible; older pages load imperatively and stay
 * in local state, deduplicated by attempt id as polling shifts the first page.
 * Unmounting the Activity tab stops Apollo polling and discards that page state.
 */
export function useWebhookDeliveries(
  endpointId: string,
  filter: WebhookDeliveryFilter = {},
): UseWebhookDeliveriesResult {
  const client = useApolloClient();
  const { currentWorkspaceId } = useWorkspace();
  const filterKey = `${currentWorkspaceId ?? ""}\n${endpointId}\n${filter.status ?? ""}\n${filter.sentAfter ?? ""}\n${filter.sentBefore ?? ""}`;
  const [pageState, setPageState] = useState<{
    key: string;
    retained: WebhookDeliveryView[];
    loadedMore: boolean;
    retainedLimit: number;
    tailFull: boolean;
  }>({
    key: filterKey,
    retained: [],
    loadedMore: false,
    retainedLimit: 0,
    tailFull: true,
  });
  const [loadingMore, setLoadingMore] = useState(false);
  const { retained, loadedMore, retainedLimit, tailFull } = useMemo(
    () =>
      pageState.key === filterKey
        ? pageState
        : {
            retained: [],
            loadedMore: false,
            retainedLimit: 0,
            tailFull: true,
          },
    [filterKey, pageState],
  );

  const { data, loading, error, refetch } = useQuery(
    WebhookDeliveriesDocument,
    {
      variables: {
        endpointId,
        ownerId: currentWorkspaceId,
        limit: PAGE_SIZE,
        status: filter.status,
        sentAfter: filter.sentAfter,
        sentBefore: filter.sentBefore,
      },
      skip: currentWorkspaceId == null,
      fetchPolicy: "network-only",
      errorPolicy: "all",
      // Only this cursor-less newest page polls. Imperatively loaded older
      // pages remain local and are reconciled by immutable attempt id below.
      pollInterval: RESOURCE_POLL_INTERVAL_MS,
      skipPollAttempt: skipPollWhenHidden,
    },
  );

  const firstPage = useMemo(() => toViews(data?.webhookDeliveries), [data]);

  // Once an older page has been loaded, retain the stitched boundary so an
  // attempt displaced from the newest page does not disappear above that
  // tail. Before then, keep the view bounded to the server's newest page.
  useEffect(() => {
    if (!data) return;
    setPageState((previous) => {
      if (firstPage.length === 0) {
        if (
          previous.key === filterKey &&
          previous.retained.length === 0 &&
          !previous.tailFull
        ) {
          return previous;
        }
        return {
          key: filterKey,
          retained: [],
          loadedMore: false,
          retainedLimit: 0,
          tailFull: false,
        };
      }

      if (previous.key !== filterKey) {
        return {
          key: filterKey,
          retained: [],
          loadedMore: false,
          retainedLimit: 0,
          tailFull: true,
        };
      }

      if (!previous.loadedMore) {
        if (previous.retained.length === 0) return previous;
        return { ...previous, retained: [] };
      }

      const next = mergeAttempts(firstPage, previous.retained).slice(
        0,
        previous.retainedLimit,
      );
      if (
        next.length === previous.retained.length &&
        next.every((attempt, index) => attempt === previous.retained[index])
      ) {
        return previous;
      }
      return {
        ...previous,
        retained: next,
      };
    });
  }, [data, filterKey, firstPage]);

  const deliveries = useMemo(() => {
    if (!loadedMore) return firstPage;
    return mergeAttempts(firstPage, retained).slice(0, retainedLimit);
  }, [firstPage, loadedMore, retained, retainedLimit]);

  const hasMore =
    deliveries.length > 0 &&
    (loadedMore ? tailFull : firstPage.length === PAGE_SIZE);

  const loadMore = useCallback(async () => {
    const tail = deliveries[deliveries.length - 1];
    if (!tail || loadingMore) return;
    setLoadingMore(true);
    try {
      const res = await client.query({
        query: WebhookDeliveriesDocument,
        variables: {
          endpointId,
          ownerId: currentWorkspaceId,
          cursor: tail.cursor,
          limit: PAGE_SIZE,
          status: filter.status,
          sentAfter: filter.sentAfter,
          sentBefore: filter.sentBefore,
        },
        fetchPolicy: "network-only",
      });
      const page = toViews(res.data?.webhookDeliveries);
      setPageState((prev) => {
        const base =
          prev.key === filterKey && prev.loadedMore ? prev.retained : firstPage;
        const next = mergeAttempts(base, page);
        return {
          key: filterKey,
          retained: next,
          loadedMore: true,
          retainedLimit: next.length,
          tailFull: page.length === PAGE_SIZE,
        };
      });
    } finally {
      setLoadingMore(false);
    }
  }, [
    client,
    deliveries,
    endpointId,
    currentWorkspaceId,
    filter.sentAfter,
    filter.sentBefore,
    filter.status,
    filterKey,
    firstPage,
    loadingMore,
  ]);

  // Manual refresh has the same reconciliation semantics as a poll: refresh
  // only the newest page and keep every keyset page the operator loaded.
  const refresh = useCallback(() => refetch(), [refetch]);

  return {
    deliveries,
    loading: loading && deliveries.length === 0,
    loadingMore,
    error,
    hasMore,
    loadMore,
    refresh,
  };
}
