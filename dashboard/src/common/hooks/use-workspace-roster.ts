import { useQuery } from "@apollo/client/react";
import {
  WorkspaceMembersDocument,
  type WorkspaceMembersQuery,
} from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export type RosterMember = NonNullable<
  NonNullable<WorkspaceMembersQuery["workspaceMembers"]>[number]
>;

/**
 * The current workspace's members, as every row that names a person resolves
 * them: one cached `WorkspaceMembers` read the whole page shares, which a
 * member who may not read it simply finds empty. `skip` holds it back when
 * the caller has nothing to look up.
 */
export function useWorkspaceRoster(skip: boolean): {
  members: readonly (RosterMember | null)[];
  loading: boolean;
} {
  const { currentWorkspaceId } = useWorkspace();
  const { data, loading } = useQuery(WorkspaceMembersDocument, {
    variables: { workspaceId: currentWorkspaceId ?? "" },
    skip: !currentWorkspaceId || skip,
    fetchPolicy: "cache-first",
    errorPolicy: "all",
  });
  return { members: data?.workspaceMembers ?? EMPTY, loading };
}

const EMPTY: readonly (RosterMember | null)[] = [];
