import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { WorkspaceMembersDocument } from "@/graphql/definitions";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  automationActions,
  type AutomationActions,
} from "@/features/agent-automations/lib/display";

export interface AutomationViewer {
  canCreate: boolean;
  canManage: boolean;
  /** The caller's identity ids, empty until the roster resolves (fail-closed). */
  identities: ReadonlySet<string>;
  /** True while the capability answer or the roster is still resolving. */
  resolving: boolean;
  /** What the caller may do with an automation created by `createdBy`. */
  actionsFor: (createdBy: string) => AutomationActions;
  isCreator: (createdBy: string) => boolean;
}

/**
 * Who the caller is for an automation: the workspace capabilities (developer
 * creates, admin manages) plus whether they created it. `createdBy` is the
 * creator's identity subject, matched against the caller's own roster row.
 */
export function useAutomationViewer(): AutomationViewer {
  const { canCreate, canManage, loaded } = useCapabilities();
  const { currentWorkspaceId } = useWorkspace();
  const { data, loading } = useQuery(WorkspaceMembersDocument, {
    variables: { workspaceId: currentWorkspaceId ?? "" },
    skip: !currentWorkspaceId,
    fetchPolicy: "cache-first",
    errorPolicy: "all",
  });
  const identities = useMemo(() => {
    const self = (data?.workspaceMembers ?? []).find((m) => m?.isSelf);
    return new Set(
      [self?.subject, self?.userId].filter((v): v is string => !!v),
    );
  }, [data]);

  return useMemo(() => {
    const isCreator = (createdBy: string) =>
      createdBy !== "" && identities.has(createdBy);
    return {
      canCreate,
      canManage,
      identities,
      resolving: !loaded || (loading && data == null),
      isCreator,
      actionsFor: (createdBy) =>
        automationActions({
          canCreate,
          canManage,
          isCreator: isCreator(createdBy),
        }),
    };
  }, [canCreate, canManage, identities, loaded, loading, data]);
}
