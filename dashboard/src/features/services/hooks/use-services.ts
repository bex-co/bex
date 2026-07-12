import { useCallback, useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { ServicesDocument } from "@/graphql/definitions";
import { toServiceViews } from "@/features/services/lib/status";
import type { ServiceView } from "@/features/services/types";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface UseServicesResult {
  services: ServiceView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the query, resolving to the fresh list (used to poll to convergence). */
  refetch: () => Promise<ServiceView[]>;
}

/**
 * Reads bex-api's `services` query and maps each Render-shaped `Service` onto a
 * normalized ServiceView. Presentation only — the list is the operator's real
 * Apps (docs/ADR006-bex-api.md); this mirrors the metrics hook's shared-Core read.
 * Scoped to the switcher's selected workspace (w6/m3): skipped until the
 * selection resolves to an id, so this never fires the fetch-then-refetch
 * pair a still-null ownerId would cause — `loading` stays true for that same
 * window, so callers don't see a flash of the empty state first.
 */
export function useServices(): UseServicesResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch } = useQuery(ServicesDocument, {
    variables: { ownerId: currentWorkspaceId },
    skip: !resolved,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  // Memoized on Apollo's `data` identity (stable between poll ticks / unrelated
  // re-renders), so the mapped list keeps a stable identity and downstream
  // `computeStats` doesn't recompute on every lifecycle-`pending` flip.
  const services = useMemo(() => toServiceViews(data?.services), [data]);

  const refetchViews = useCallback(async () => {
    const res = await refetch();
    return toServiceViews(res.data?.services);
  }, [refetch]);

  return { services, loading: !resolved || loading, error, refetch: refetchViews };
}
