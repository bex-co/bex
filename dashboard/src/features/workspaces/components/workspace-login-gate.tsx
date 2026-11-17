import { useEffect } from "react";
import { useLocation } from "@tanstack/react-router";
import { WorkspaceGoogleLoginBlockedPage } from "@/features/workspaces/components/workspace-google-login-blocked-page";
import { WorkspaceTwoFactorBlockedPage } from "@/features/workspaces/components/workspace-two-factor-blocked-page";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useWorkspaceLoginBlock } from "@/features/workspaces/hooks/use-workspace-login-block";
import {
  clearWorkspaceLoginBlocks,
  isLoginRequirementExemptPath,
} from "@/features/workspaces/lib/workspace-login-block";

/**
 * Walls the selected workspace behind ONE explanatory page when bex-api has
 * refused it for a login requirement: `WORKSPACE_2FA_REQUIRED` (w22/m402,
 * docs/ADR012-auth.md 9a) or `WORKSPACE_GOOGLE_LOGIN_REQUIRED` (w7/m465, 9b),
 * instead of every query on the page failing on its own. The refusal is read
 * in one place, the Apollo auth error link, which records the workspace and
 * the requirement in `workspace-login-block`; this gate compares that record
 * with the selection, so a refusal from another workspace walls nothing until
 * it is selected, and shows the page of the requirement the server named.
 *
 * Two exits keep it from becoming a trap. The account pages where a member
 * enrolls or connects Google (and the create-workspace page) are never walled,
 * and the shell around the gate (sidebar, workspace switcher) stays mounted.
 * And a recorded block is dropped on leaving those pages, since the member may
 * just have enrolled: the page's own queries then decide again, and a blocked
 * page returns only if the server still refuses.
 *
 * While a page is up, the routed content is unmounted, so nothing under it
 * keeps querying a workspace that will only refuse.
 */
export function WorkspaceLoginGate({
  children,
}: {
  children: React.ReactNode;
}) {
  const { currentWorkspaceId } = useWorkspace();
  const blocked = useWorkspaceLoginBlock(currentWorkspaceId);
  const exempt = useLocation({
    select: (location) => isLoginRequirementExemptPath(location.pathname),
  });

  useEffect(() => {
    if (!exempt) return;
    return clearWorkspaceLoginBlocks;
  }, [exempt]);

  if (blocked === null || exempt || currentWorkspaceId === null) {
    return children;
  }
  return blocked === "googleLogin" ? (
    <WorkspaceGoogleLoginBlockedPage workspaceId={currentWorkspaceId} />
  ) : (
    <WorkspaceTwoFactorBlockedPage workspaceId={currentWorkspaceId} />
  );
}
