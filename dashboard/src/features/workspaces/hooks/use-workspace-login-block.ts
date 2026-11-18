import { useSyncExternalStore } from "react";
import {
  organizationLoginBlock,
  subscribeLoginBlocks,
  workspaceLoginBlock,
  type LoginBlock,
  type OrganizationLoginBlock,
} from "@/features/workspaces/lib/workspace-login-block";

/** One recorded block, re-read whenever the record changes; null on the
 *  server, which never holds one. */
function useLoginBlock<T extends LoginBlock>(read: () => T | null): T | null {
  return useSyncExternalStore(subscribeLoginBlocks, read, () => null);
}

/**
 * Which login requirement bex-api has refused the caller `workspaceId` for
 * (w22/m402, w7/m465), and the org whose setting it was (w7/m466), or null.
 * Null without a workspace, and on the server.
 */
export function useWorkspaceLoginBlock(
  workspaceId: string | null,
): LoginBlock | null {
  return useLoginBlock(() =>
    workspaceId === null ? null : workspaceLoginBlock(workspaceId),
  );
}

/**
 * Which login requirement the org's own setting has refused the caller for
 * (w7/m466), on the org's pages or in one of its workspaces, or null. Null on
 * the server.
 */
export function useOrganizationLoginBlock(
  organizationId: string,
): OrganizationLoginBlock | null {
  return useLoginBlock(() => organizationLoginBlock(organizationId));
}
