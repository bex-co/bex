import { useSyncExternalStore } from "react";
import {
  isWorkspaceTwoFactorBlocked,
  subscribeWorkspaceTwoFactorBlocks,
} from "@/features/workspaces/lib/two-factor-block";

/**
 * Whether bex-api has refused the caller `workspaceId` for want of a second
 * factor (w22/m402). False without a workspace, and on the server.
 */
export function useWorkspaceTwoFactorBlocked(
  workspaceId: string | null,
): boolean {
  return useSyncExternalStore(
    subscribeWorkspaceTwoFactorBlocks,
    () => workspaceId !== null && isWorkspaceTwoFactorBlocked(workspaceId),
    () => false,
  );
}
