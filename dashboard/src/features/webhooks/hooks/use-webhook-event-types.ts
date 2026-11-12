import { useCallback, useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { WebhookEventTypesDocument } from "@/graphql/definitions";
import { pickerEventTypes } from "@/features/webhooks/event-catalog";

export interface UseWebhookEventTypesResult {
  eventTypes: string[];
  loading: boolean;
  error: Error | undefined;
  retry: () => Promise<unknown>;
}

/**
 * Reads the server's subscribable event-type vocabulary (`webhookEventTypes`)
 * — the create form's checkbox list. Served, not hardcoded, so the picker
 * can't drift from what bex-api actually emits. The types Render's picker
 * leaves to the API (the edge-cache three) are dropped here, so neither the
 * list nor "All events" offers them; a subscription that already names one
 * keeps it on save.
 */
export function useWebhookEventTypes(): UseWebhookEventTypesResult {
  const { data, loading, error, refetch } = useQuery(
    WebhookEventTypesDocument,
    {
      fetchPolicy: "cache-first",
      errorPolicy: "all",
      notifyOnNetworkStatusChange: true,
    },
  );
  const eventTypes = useMemo(
    () =>
      pickerEventTypes(
        (data?.webhookEventTypes ?? []).filter((t): t is string => !!t),
      ),
    [data],
  );
  const retry = useCallback(() => refetch(), [refetch]);
  return { eventTypes, loading, error, retry };
}
