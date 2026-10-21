import { useEffect, useRef } from "react";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/**
 * Make a resource's owning workspace the selection when its detail page opens
 * (w6/091). Capabilities and resource-action projections are asked about the
 * *selected* workspace, so a service opened from another workspace — a direct
 * link, search, a notification — showed its member's allowed actions as denied
 * until they used the switcher.
 *
 * Membership-checked against the switcher's list, as `/w/$` does: a
 * non-member's selection never changes (their page fails closed as before).
 * Adopts at most once per resource, so a deliberate switch from the page — the
 * switcher then navigates home — is never undone while this page unmounts.
 */
export function useAdoptResourceWorkspace(
  resourceId: string,
  ownerId: string | null | undefined,
) {
  const { workspaces, loading, currentWorkspaceId, setCurrentWorkspaceId } =
    useWorkspace();
  const adoptedFor = useRef<string | null>(null);

  useEffect(() => {
    if (!ownerId || loading || adoptedFor.current === resourceId) return;
    if (!workspaces.some((w) => w.id === ownerId)) return;
    adoptedFor.current = resourceId;
    if (ownerId !== currentWorkspaceId) setCurrentWorkspaceId(ownerId);
  }, [
    resourceId,
    ownerId,
    loading,
    workspaces,
    currentWorkspaceId,
    setCurrentWorkspaceId,
  ]);
}
