import { useWorkspace } from "@/features/workspaces/context/hooks";
import { proFeaturesPlan } from "@/features/workspaces/types";

/** Whether the current workspace is known to be below the audit-log plan
 *  (Render: Pro and higher, w8/m138), so a loading Audit Log previews its plan
 *  notice (w4/684). An unknown plan previews the table. */
export function useAuditPlanKnownMissing(): boolean {
  const { currentWorkspace } = useWorkspace();
  return !!currentWorkspace && !proFeaturesPlan(currentWorkspace.plan);
}
