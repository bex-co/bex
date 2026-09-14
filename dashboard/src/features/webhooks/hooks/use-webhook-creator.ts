import { useWorkspaceSubjectLabel } from "@/common/hooks/use-workspace-subject-label";

/** @deprecated Prefer useWorkspaceSubjectLabel — kept as a thin alias for webhook call sites. */
export function useWebhookCreator(subject: string): string {
  return useWorkspaceSubjectLabel(subject);
}
