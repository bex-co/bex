import { useQuery } from "@apollo/client/react";
import { RouterAvailableDocument } from "@/graphql/definitions";
import { useRouterFeatureEnabled } from "@/config/use-growthbook";
import { useWorkspace } from "@/features/workspaces/context";

/**
 * Whether the workspace is offered Router: its flag is on AND the optional
 * companion service answers `routerAvailable` (w21/042). The sidebar and the ⌘K
 * palette both ask this, so neither lists a destination `routes/router.tsx`
 * would bounce to the overview. Only flagged workspaces probe the companion
 * service, and the answer is not trusted until it arrives (no Router row while
 * it loads); both callers share Apollo's cache, so it is one request.
 */
export function useRouterNavAvailable(): boolean {
  const routerEnabled = useRouterFeatureEnabled();
  const { currentWorkspaceId } = useWorkspace();
  const { data } = useQuery(RouterAvailableDocument, {
    variables: { ownerId: currentWorkspaceId ?? "" },
    skip: !routerEnabled,
  });
  return routerEnabled && data?.routerAvailable === true;
}
