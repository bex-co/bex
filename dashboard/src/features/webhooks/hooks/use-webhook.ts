import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  WebhookEndpointDocument,
  type WebhookEndpointQuery,
} from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import type { WebhookEndpointView } from "@/features/webhooks/types";

function toView(
  raw: WebhookEndpointQuery["webhookEndpoint"] | undefined,
): WebhookEndpointView | null {
  if (!raw?.id) return null;
  return {
    id: raw.id,
    name: raw.name ?? "",
    ownerId: raw.ownerId || null,
    url: raw.url ?? "",
    eventTypes: (raw.eventTypes ?? []).filter((t): t is string => !!t),
    enabled: raw.enabled ?? false,
    disabledReason: raw.disabledReason ?? "",
    secret: raw.secret ?? "",
    createdAt: raw.createdAt,
    createdBy: raw.createdBy ?? "",
    latestStatus: "",
    latestSentAt: null,
    latestParentStatus: "",
  };
}

export interface UseWebhookResult {
  endpoint: WebhookEndpointView | null;
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
}

/**
 * One endpoint by id for the `/webhook/$webhookId` detail page (w1/m49).
 * Resolved by id in its own workspace, with the signing secret bex-api
 * returns to workspace admins (w3/m282). An unknown or foreign id settles
 * empty, and the page draws its state for the read (`resourceReadState`,
 * w3/m706) — never a crash.
 */
export function useWebhook(id: string): UseWebhookResult {
  const { data, loading, error, refetch } = useQuery(WebhookEndpointDocument, {
    // No ownerId: a webhook id resolves its own workspace server-side, so a
    // link to one in another of the caller's workspaces opens (w4/443, as
    // w4/m169 did for Blueprints).
    variables: { id },
    // The parent route's network-only loader fills this exact query for the
    // SSR title and shell. Consume that normalized result instead of issuing a
    // second request solely because both the head and page need the endpoint.
    fetchPolicy: "cache-first",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });

  const endpoint = useMemo(() => toView(data?.webhookEndpoint), [data]);

  return { endpoint, loading, error, refetch };
}
