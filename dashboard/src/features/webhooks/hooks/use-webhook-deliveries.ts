import { useCallback, useMemo, useState } from "react";
import { useApolloClient, useQuery } from "@apollo/client/react";
import {
  WebhookDeliveriesDocument,
  type WebhookDeliveriesQuery,
} from "@/graphql/definitions";
import type {
  WebhookDeliveryView,
  WebhookDeliveryStatus,
} from "@/features/webhooks/types";
import { useWorkspace } from "@/features/workspaces/context/hooks";

const PAGE_SIZE = 20;

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
 * Mounted inside the per-endpoint history dialog, so all paging state is
 * naturally discarded on close.
 */
export function useWebhookDeliveries(
  endpointId: string,
): UseWebhookDeliveriesResult {
  const client = useApolloClient();
  const { currentWorkspaceId } = useWorkspace();
  const [appended, setAppended] = useState<WebhookDeliveryView[]>([]);
  const [loadingMore, setLoadingMore] = useState(false);
  const [tailFull, setTailFull] = useState(true);

  const { data, loading, error, refetch } = useQuery(
    WebhookDeliveriesDocument,
    {
      variables: {
        endpointId,
        ownerId: currentWorkspaceId,
        limit: PAGE_SIZE,
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
        },
        fetchPolicy: "network-only",
      });
      const page = toViews(res.data?.webhookDeliveries);
      setAppended((prev) => [...prev, ...page]);
      setTailFull(page.length === PAGE_SIZE);
    } finally {
      setLoadingMore(false);
    }
  }, [client, deliveries, endpointId, currentWorkspaceId, loadingMore]);

  const refresh = useCallback(async () => {
    setAppended([]);
    setTailFull(true);
    return refetch();
  }, [refetch]);

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
