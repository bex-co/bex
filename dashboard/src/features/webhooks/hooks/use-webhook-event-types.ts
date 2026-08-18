import { useCallback, useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { WebhookEventTypesDocument } from "@/graphql/definitions";

export interface UseWebhookEventTypesResult {
  eventTypes: string[];
  loading: boolean;
  error: Error | undefined;
  retry: () => Promise<unknown>;
}

/**
 * Reads the server's subscribable event-type vocabulary (`webhookEventTypes`)
 * — the create form's checkbox list. Served, not hardcoded, so the picker
 * can't drift from what bex-api actually emits.
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
    () => (data?.webhookEventTypes ?? []).filter((t): t is string => !!t),
    [data],
  );
  const retry = useCallback(() => refetch(), [refetch]);
  return { eventTypes, loading, error, retry };
}
