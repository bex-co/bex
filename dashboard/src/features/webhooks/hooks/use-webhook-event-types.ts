import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { WebhookEventTypesDocument } from "@/graphql/definitions";

export interface UseWebhookEventTypesResult {
  eventTypes: string[];
  loading: boolean;
}

/**
 * Reads the server's subscribable event-type vocabulary (`webhookEventTypes`)
 * — the create form's checkbox list. Served, not hardcoded, so the picker
 * can't drift from what bex-api actually emits.
 */
export function useWebhookEventTypes(): UseWebhookEventTypesResult {
  const { data, loading } = useQuery(WebhookEventTypesDocument, {
    fetchPolicy: "cache-first",
  });
  const eventTypes = useMemo(
    () => (data?.webhookEventTypes ?? []).filter((t): t is string => !!t),
    [data],
  );
  return { eventTypes, loading };
}
