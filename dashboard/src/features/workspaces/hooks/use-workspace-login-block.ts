import { useSyncExternalStore } from "react";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import {
  subscribeWorkspaceLoginBlocks,
  workspaceLoginBlock,
} from "@/features/workspaces/lib/workspace-login-block";

/**
 * Which login requirement bex-api has refused the caller `workspaceId` for
 * (w22/m402, w7/m465), or null. Null without a workspace, and on the server.
 */
export function useWorkspaceLoginBlock(
  workspaceId: string | null,
): WorkspaceLoginRequirement | null {
  return useSyncExternalStore(
    subscribeWorkspaceLoginBlocks,
    () => (workspaceId === null ? null : workspaceLoginBlock(workspaceId)),
    () => null,
  );
}
