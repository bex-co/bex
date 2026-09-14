import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  WorkspaceMembersDocument,
  type WorkspaceMembersQuery,
} from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/**
 * Resolve a stored identity subject (audit actor, API-key createdBy, webhook
 * createdBy, deploy triggeredByUser) through the authorized workspace roster.
 * Falls back to the raw subject when the member list has no match (w4/069).
 */
export function useWorkspaceSubjectLabel(subject: string): string {
  const { currentWorkspaceId } = useWorkspace();
  const { data } = useQuery(WorkspaceMembersDocument, {
    variables: { workspaceId: currentWorkspaceId ?? "" },
    skip: !currentWorkspaceId || !subject,
    fetchPolicy: "cache-first",
    errorPolicy: "all",
  });

  return useMemo(() => {
    const members: NonNullable<WorkspaceMembersQuery["workspaceMembers"]> =
      data?.workspaceMembers ?? [];
    const member = members.find(
      (candidate) =>
        candidate?.subject === subject || candidate?.userId === subject,
    );
    return member?.email || member?.userId || subject;
  }, [data, subject]);
}
