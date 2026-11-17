import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  organizationPlanOwner,
  proFeaturesPlan,
  type OrganizationPlanOwner,
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
 *  kept until the admin grant loads, as Build Pipeline is. */
export function useWorkspaceSettingsSkeletonSections(): WorkspaceSettingsPendingSections {
  const { currentWorkspace, workspaces } = useWorkspace();
  const capabilities = useCapabilities();
  return {
    buildPipeline: capabilities.canManageBilling || capabilities.loading,
    auditLogs: !currentWorkspace || currentWorkspace.role === "admin",
    deleteWorkspace: workspaces.length > 1,
    privacy:
      currentWorkspace?.organization != null &&
      (capabilities.canManage || capabilities.loading),
    planOwner: organizationPlanOwner(currentWorkspace, workspaces),
    workspaceName: currentWorkspace?.name,
    belowPro:
      currentWorkspace != null && !proFeaturesPlan(currentWorkspace.plan),
  };
}
