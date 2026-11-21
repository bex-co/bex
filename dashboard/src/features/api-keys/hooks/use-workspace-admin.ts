import { useState } from "react";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/**
 * Whether the caller is an Admin of the selected workspace (`can_manage`), as
 * last answered there: null until the first answer. A capability refresh on
 * focus or reconnect withdraws `canManage` until it answers, which must not
 * blink the API keys panel's Admin switch away (w9/m539). Presentation only:
 * bex-api decides `allMembers` itself.
 */
export function useWorkspaceAdmin(): boolean | null {
  const { canManage, loaded } = useCapabilities();
  const { currentWorkspaceId } = useWorkspace();
  const [last, setLast] = useState<{
    workspaceId: string | null;
    admin: boolean;
  } | null>(null);
  if (
    loaded &&
    (last?.workspaceId !== currentWorkspaceId || last.admin !== canManage)
  ) {
    setLast({ workspaceId: currentWorkspaceId, admin: canManage });
  }
  if (loaded) return canManage;
  return last?.workspaceId === currentWorkspaceId ? last.admin : null;
}
