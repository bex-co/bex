import { useCallback } from "react";
import { useQuery } from "@apollo/client/react";
import { ServerDocument } from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { toServiceView } from "@/features/services/lib/status";
import type { ServiceView } from "@/features/services/types";

export interface UseServerOptions {
  /**
   * Poll for out-of-band changes (default `true`). Pass `false` on a secondary
   * consumer mounted alongside a polling one: every `useQuery` gets its own
   * timer, and two timers reschedule off their own responses, so they drift
   * apart into separate round trips instead of deduplicating. The designated
   * polling owner is the service-detail layout (service-detail-layout.tsx);
   * the sidebar, breadcrumbs, and tab pages all read the same cached document.
   */
  poll?: boolean;
}

export interface UseServerResult {
  /** The service, or null while loading / when no such App exists. */
  service: ServiceView | null;
  loading: boolean;
  error: Error | undefined;
  /**
   * Re-run the query, resolving the fresh view as a one-element list (empty when
   * the App is gone). Shaped as a list so it plugs straight into
   * `useServiceLifecycle`'s poll-to-converge, which finds by id.
   */
  refetch: () => Promise<ServiceView[]>;
}

/**
 * Reads bex-api's `server(id)` query (Render's dashboard name for a single
 * service) and maps the Render-shaped `Service` onto a normalized ServiceView.
 * Presentation only — the same shared Core read the `services` list uses
 * (docs/ADR006-bex-api.md); mirrors `useServices` for one App.
 */
export function useServer(
  id: string,
  { poll = true }: UseServerOptions = {},
): UseServerResult {
  const { data, loading, error, refetch } = useQuery(ServerDocument, {
    variables: { id },
    fetchPolicy: "cache-first",
    errorPolicy: "all",
    pollInterval: poll ? RESOURCE_POLL_INTERVAL_MS : 0,
    skipPollAttempt: skipPollWhenHidden,
  });

  const service = data?.server ? toServiceView(data.server) : null;

  const refetchViews = useCallback(async () => {
    const res = await refetch();
    return res.data?.server ? [toServiceView(res.data.server)] : [];
  }, [refetch]);

  return { service, loading, error, refetch: refetchViews };
}
