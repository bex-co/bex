import { useMemo } from "react";
import { useSelfSubjects } from "@/common/hooks/use-self-subjects";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
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
  const { subjects: identities, resolving } = useSelfSubjects();

  return useMemo(() => {
    const isCreator = (createdBy: string) =>
      createdBy !== "" && identities.has(createdBy);
    return {
      canCreate,
      canManage,
      identities,
      resolving: !loaded || resolving,
      isCreator,
      actionsFor: (createdBy) =>
        automationActions({
          canCreate,
          canManage,
          isCreator: isCreator(createdBy),
        }),
    };
  }, [canCreate, canManage, identities, loaded, resolving]);
}
