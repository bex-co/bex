import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { WorkspaceMembersDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface SelfSubjects {
  /** The caller's identity ids, empty until the roster resolves (fail-closed). */
  subjects: ReadonlySet<string>;
  /** True while the roster has not answered yet. */
  resolving: boolean;
}

/**
 * Who the caller is in the selected workspace: the subject and user id of
 * their own roster row, which a resource's `createdBy` (an identity subject)
 * is matched against to tell the caller's own automations and API keys.
 */
export function useSelfSubjects(): SelfSubjects {
  const { currentWorkspaceId } = useWorkspace();
  const { data, loading } = useQuery(WorkspaceMembersDocument, {
    variables: { workspaceId: currentWorkspaceId ?? "" },
    skip: !currentWorkspaceId,
    fetchPolicy: "cache-first",
    errorPolicy: "all",
  });
  return useMemo(() => {
    const self = (data?.workspaceMembers ?? []).find((m) => m?.isSelf);
    return {
      subjects: new Set(
        [self?.subject, self?.userId].filter((v): v is string => !!v),
      ),
      resolving: loading && data == null,
    };
  }, [data, loading]);
}
