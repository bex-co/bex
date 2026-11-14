import { useEffect } from "react";
import { useLocation } from "@tanstack/react-router";
import { WorkspaceTwoFactorBlockedPage } from "@/features/workspaces/components/workspace-two-factor-blocked-page";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useWorkspaceTwoFactorBlocked } from "@/features/workspaces/hooks/use-workspace-two-factor-blocked";
import {
  clearWorkspaceTwoFactorBlocks,
  isTwoFactorExemptPath,
} from "@/features/workspaces/lib/two-factor-block";

/**
 * Walls the selected workspace behind ONE explanatory page when bex-api has
 * refused it with `WORKSPACE_2FA_REQUIRED` (w22/m402, docs/ADR012-auth.md 9a),
 * instead of every query on the page failing on its own. The refusal is read
 * in one place, the Apollo auth error link, which records the workspace in
 * `two-factor-block`; this gate compares that record with the selection, so a
 * refusal from another workspace walls nothing until it is selected.
 *
 * Two exits keep it from becoming a trap. The account pages where a member
 * enrolls (and the create-workspace page) are never walled, and the shell
 * around the gate (sidebar, workspace switcher) stays mounted. And a recorded
 * block is dropped on leaving those pages, since the member may just have
 * enrolled: the page's own queries then decide again, and this page returns
 * only if the server still refuses.
 *
 * While the page is up, the routed content is unmounted, so nothing under it
 * keeps querying a workspace that will only refuse.
 */
export function WorkspaceTwoFactorGate({
  children,
}: {
  children: React.ReactNode;
}) {
  const { currentWorkspaceId } = useWorkspace();
  const blocked = useWorkspaceTwoFactorBlocked(currentWorkspaceId);
  const exempt = useLocation({
    select: (location) => isTwoFactorExemptPath(location.pathname),
  });

  useEffect(() => {
    if (!exempt) return;
    return clearWorkspaceTwoFactorBlocks;
  }, [exempt]);

  if (blocked && !exempt && currentWorkspaceId !== null) {
    return <WorkspaceTwoFactorBlockedPage workspaceId={currentWorkspaceId} />;
  }
  return children;
}
