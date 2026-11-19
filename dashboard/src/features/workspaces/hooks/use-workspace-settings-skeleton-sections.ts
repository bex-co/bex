import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { workspaceRequires } from "@/features/workspaces/lib/login-requirements";
import {
  getPersistedWorkspaceShape,
  workspaceShape,
} from "@/features/workspaces/lib/selection";
import {
  inboundIpRulesPlan,
  proFeaturesPlan,
  type OrganizationPlanOwner,
  type WorkspaceView,
} from "@/features/workspaces/types";

/** The conditional sections a Workspace Settings pending frame draws. */
export interface WorkspaceSettingsPendingSections {
  buildPipeline: boolean;
  auditLogs: boolean;
  deleteWorkspace: boolean;
  privacy: boolean;
  /** The General card's org-plan note on a secondary workspace (w7/m464). */
  planOwner: OrganizationPlanOwner | null;
  workspaceName: string | undefined;
  /** The workspace is known to be below Pro, so the 2FA and Google login
   *  cards carry their plan notice (w7/070, w7/m465). */
  belowPro: boolean;
  /** The workspace is known to be below Scale, so Inbound IP Rules is the
   *  read-only view with its plan notice, not the editor (w7/094). */
  belowScale: boolean;
  /** Whether the caller manages the selected workspace (`can_manage`), as
   *  the ready cards read it, or null while capabilities are checking. A
   *  non-admin's cards draw the Admin-only notes in place of the Admin's
   *  editors, Save buttons and Invite, and no Audit Logs (w7/094); an
   *  unknown role keeps the Admin's frame. */
  canManage: boolean | null;
  /** The workspace's org, once known: an org that requires 2FA or Google
   *  login gives that card its "required by" note (w7/m466). */
  organization: WorkspaceView["organization"];
  /** Whether the workspace requires 2FA and Google login, once the caller is
   *  known not to be an admin of a workspace already loaded: those cards then
   *  show the state as a badge, not a switch, so they hold the badge's box
   *  (w7/089). Null while the caller may be an admin or the workspace is
   *  not loaded. */
  loginRequirementStatus: Readonly<
    Record<WorkspaceLoginRequirement, boolean>
  > | null;
}

/** Which of Workspace Settings' conditional sections a pending frame
 *  previews (w3/m305), from what the shell already knows: Build Pipeline for
 *  billing managers, Audit Logs for admins (both kept until their deciding
 *  data loads, as the inline skeleton always did) and the Delete card when
 *  there is another workspace to land in.
 *
 *  Workspace privacy and the org-plan note (w7/m464) belong to an org
 *  workspace, so they wait for the workspace itself: most workspaces are
 *  standalone, and keeping them while it loads would draw for nearly everyone
 *  a card the ready page then drops. Once the workspace is known, privacy is
 *  kept until the admin grant loads, as Build Pipeline is.
 *
 *  Before the list arrives (a direct load), the workspace is what the last
 *  visit stored beside the selection cookie (w3/073), so an org admin's frame
 *  draws privacy and the Delete card too. A first visit has nothing stored
 *  and keeps the standalone default. */
export function useWorkspaceSettingsSkeletonSections(): WorkspaceSettingsPendingSections {
  const { currentWorkspace, currentWorkspaceId, workspaces } = useWorkspace();
  const capabilities = useCapabilities();
  const shape = currentWorkspace
    ? workspaceShape(currentWorkspace, workspaces)
    : workspaces.length === 0
      ? getPersistedWorkspaceShape(currentWorkspaceId)
      : null;
  const canManage = capabilities.loading ? null : capabilities.canManage;
  return {
    buildPipeline: capabilities.canManageBilling || capabilities.loading,
    // Audit logs are `can_manage`'s; the list's role is "admin" for every
    // member, so it decides only until capabilities do (w7/094).
    auditLogs: canManage !== false && (!shape || shape.role === "admin"),
    deleteWorkspace: shape?.otherWorkspaces ?? workspaces.length > 1,
    privacy: shape?.organization != null && canManage !== false,
    planOwner: shape?.planOwner ?? null,
    workspaceName: currentWorkspace?.name,
    belowPro: shape != null && !proFeaturesPlan(shape.plan),
    belowScale: shape != null && !inboundIpRulesPlan(shape.plan),
    canManage,
    organization: shape?.organization ?? null,
    loginRequirementStatus:
      currentWorkspace && canManage === false
        ? {
            twoFactor: workspaceRequires("twoFactor", currentWorkspace),
            googleLogin: workspaceRequires("googleLogin", currentWorkspace),
          }
        : null,
  };
}
