import { useMemo } from "react";
import { useWorkspaceRoster } from "@/common/hooks/use-workspace-roster";

/**
 * What a subject reads while the member list is still loading (w4/602): a
 * short placeholder, never the raw id it may resolve away from.
 */
export const SUBJECT_LABEL_PENDING = "…";

/**
 * Resolve a stored identity subject (audit actor, API-key createdBy, webhook
 * createdBy, deploy triggeredByUser) through the authorized workspace roster.
 * A `known` label the caller already holds (an event's own user email) wins
 * from first paint. Until the roster settles an unresolved subject reads
 * SUBJECT_LABEL_PENDING; once it settles without a match it falls back to the
 * raw subject, a removed member's id (w4/069, w21/014).
 */
export function useWorkspaceSubjectLabel(
  subject: string,
  known?: string | null,
): string {
  const { members, loading } = useWorkspaceRoster(!subject || !!known);

  return useMemo(() => {
    if (known) return known;
    const member = members.find(
      (candidate) =>
        candidate?.subject === subject || candidate?.userId === subject,
    );
    const label = member?.email || member?.userId;
    if (label) return label;
    return loading ? SUBJECT_LABEL_PENDING : subject;
  }, [members, loading, subject, known]);
}
