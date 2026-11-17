import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/** The conditional sections a Workspace Settings pending frame draws. */
export interface WorkspaceSettingsPendingSections {
  buildPipeline: boolean;
  auditLogs: boolean;
  deleteWorkspace: boolean;
  workspaceName: string | undefined;
}

/** Which of Workspace Settings' conditional sections a pending frame
 *  previews (w3/m305), from what the shell already knows: Build Pipeline for
 *  billing managers, Audit Logs for admins (both kept until their deciding
 *  data loads, as the inline skeleton always did) and the Delete card when
 *  there is another workspace to land in. */
export function useWorkspaceSettingsSkeletonSections(): WorkspaceSettingsPendingSections {
  const { currentWorkspace, workspaces } = useWorkspace();
  const capabilities = useCapabilities();
  return {
    buildPipeline: capabilities.canManageBilling || capabilities.loading,
    auditLogs: !currentWorkspace || currentWorkspace.role === "admin",
    deleteWorkspace: workspaces.length > 1,
    workspaceName: currentWorkspace?.name,
  };
}
