import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  getPersistedWorkspaceShape,
  workspaceShape,
} from "@/features/workspaces/lib/selection";
import {
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
  /** The workspace's org, once known: an org that requires 2FA or Google
   *  login gives that card its "required by" note (w7/m466). */
  organization: WorkspaceView["organization"];
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
  return {
    buildPipeline: capabilities.canManageBilling || capabilities.loading,
    auditLogs: !shape || shape.role === "admin",
    deleteWorkspace: shape?.otherWorkspaces ?? workspaces.length > 1,
    privacy:
      shape?.organization != null &&
      (capabilities.canManage || capabilities.loading),
    planOwner: shape?.planOwner ?? null,
    workspaceName: currentWorkspace?.name,
    belowPro: shape != null && !proFeaturesPlan(shape.plan),
    organization: shape?.organization ?? null,
  };
}
