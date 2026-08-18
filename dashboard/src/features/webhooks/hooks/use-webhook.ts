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
import { useWorkspace } from "@/features/workspaces/context/hooks";

function toView(
  raw: WebhookEndpointQuery["webhookEndpoint"] | undefined,
): WebhookEndpointView | null {
  if (!raw?.id) return null;
  return {
    id: raw.id,
    name: raw.name ?? "",
    url: raw.url ?? "",
    eventTypes: (raw.eventTypes ?? []).filter((t): t is string => !!t),
    enabled: raw.enabled ?? false,
    disabledReason: raw.disabledReason ?? "",
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
  /** True once the query settled without finding the endpoint (bad/foreign id). */
  notFound: boolean;
  refetch: () => Promise<unknown>;
}

/**
 * One endpoint by id for the `/webhook/$webhookId` detail page (w1/m49).
 * Workspace-scoped like every webhook read; the signing secret is never
 * requested (mint-once contract, t006). An unknown or foreign id resolves to
 * `notFound`, which the page renders as its 404 state — never a crash.
 */
export function useWebhook(id: string): UseWebhookResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch } = useQuery(WebhookEndpointDocument, {
    variables: { id, ownerId: currentWorkspaceId },
    skip: !resolved,
    // The parent route's network-only loader fills this exact query for the
    // SSR title and shell. Consume that normalized result instead of issuing a
    // second request solely because both the head and page need the endpoint.
    fetchPolicy: "cache-first",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });

  const endpoint = useMemo(() => toView(data?.webhookEndpoint), [data]);
  const settled = resolved && !loading;

  return {
    endpoint,
    loading: !resolved || loading,
    error,
    notFound: settled && endpoint === null,
    refetch,
  };
}
