import { useCallback, useMemo, useState } from "react";
import { useApolloClient, useQuery } from "@apollo/client/react";
import { type WebhookDeliveriesQuery } from "@/graphql/definitions";
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
      eventType: d.eventType ?? "",
      serviceId: d.serviceId ?? "",
      status: (d.status ?? "pending") as WebhookDeliveryStatus,
      attemptCount: d.attemptCount ?? 0,
      lastStatusCode: d.lastStatusCode ?? 0,
      lastError: d.lastError ?? "",
      responseBody: d.responseBody ?? "",
      sentAt: d.sentAt,
      nextAttemptAt: d.nextAttemptAt,
      deliveredAt: d.deliveredAt,
      createdAt: d.createdAt,
      cursor: d.cursor ?? "",
    }));
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
 * One endpoint's delivery history, newest first, keyset-cursor paged (the
 * audit-log pattern, w4/m14): the first page via useQuery, further pages
 * appended imperatively — there's no Apollo field policy merging this list.
 * Paging state lives here, so it is naturally discarded when the endpoint's
 * Activity tab unmounts.
 */
export function useWebhookDeliveries(
  endpointId: string,
  filter: WebhookDeliveryFilter = {},
): UseWebhookDeliveriesResult {
  const client = useApolloClient();
  const { currentWorkspaceId } = useWorkspace();
  const filterKey = `${filter.status ?? ""}\n${filter.sentAfter ?? ""}\n${filter.sentBefore ?? ""}`;
  const [pageState, setPageState] = useState<{
    key: string;
    appended: WebhookDeliveryView[];
    tailFull: boolean;
  }>({ key: filterKey, appended: [], tailFull: true });
  const [loadingMore, setLoadingMore] = useState(false);
  const { appended, tailFull } = useMemo(
    () =>
      pageState.key === filterKey
        ? pageState
        : { appended: [], tailFull: true },
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
    },
  );

  const firstPage = useMemo(() => toViews(data?.webhookDeliveries), [data]);
  const deliveries = useMemo(() => {
    const seen = new Set(firstPage.map((d) => d.id));
    return [...firstPage, ...appended.filter((d) => !seen.has(d.id))];
  }, [firstPage, appended]);

  const hasMore =
    deliveries.length > 0 &&
    (appended.length > 0 ? tailFull : firstPage.length === PAGE_SIZE);

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
      setPageState((prev) => ({
        key: filterKey,
        appended:
          prev.key === filterKey ? [...prev.appended, ...page] : page,
        tailFull: page.length === PAGE_SIZE,
      }));
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
    loadingMore,
  ]);

  const refresh = useCallback(async () => {
    setPageState({ key: filterKey, appended: [], tailFull: true });
    return refetch();
  }, [filterKey, refetch]);

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
