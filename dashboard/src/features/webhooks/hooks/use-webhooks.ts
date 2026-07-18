import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  WebhookEndpointsDocument,
  type WebhookEndpointsQuery,
} from "@/graphql/definitions";
import type { WebhookEndpointView } from "@/features/webhooks/types";
import { useWorkspace } from "@/features/workspaces/context/hooks";

// Derived from the generated query so new fields can't drift from the schema.
type RawEndpoint = NonNullable<
  WebhookEndpointsQuery["webhookEndpoints"]
>[number];

function toViews(
  raw: WebhookEndpointsQuery["webhookEndpoints"] | undefined,
): WebhookEndpointView[] {
  return (raw ?? [])
    .filter((e): e is NonNullable<RawEndpoint> & { id: string } => !!e?.id)
    .map((e) => ({
      id: e.id,
      name: e.name ?? "",
      url: e.url ?? "",
      eventTypes: (e.eventTypes ?? []).filter((t): t is string => !!t),
      enabled: e.enabled ?? false,
      disabledReason: e.disabledReason ?? "",
      createdAt: e.createdAt,
      createdBy: "", // list rows never request the creator; the detail query does
    }));
}

export interface UseWebhooksResult {
  endpoints: WebhookEndpointView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the list query (callers refresh after create/toggle/delete). */
  refetch: () => Promise<unknown>;
}

/**
 * Reads bex-api's `webhookEndpoints` — the workspace's outbound-webhook
 * destinations (w3/m11). Signing secrets are never requested here (see the
 * feature's `.graphql` file); only the create mutation ever returns one.
 * Scoped to the switcher's selected workspace, mirroring useApiKeys.
 */
export function useWebhooks(): UseWebhooksResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch } = useQuery(WebhookEndpointsDocument, {
    variables: { ownerId: currentWorkspaceId },
    skip: !resolved,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const endpoints = useMemo(() => toViews(data?.webhookEndpoints), [data]);

  return { endpoints, loading: !resolved || loading, error, refetch };
}
