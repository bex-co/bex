import { useQuery } from "@apollo/client/react";
import { WorkspaceLimitsDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/**
 * Whether the current workspace's pipeline minutes are spent (w3/108): its
 * builds are stopped for the month, and Render's dashboard disables Manual
 * Build and Manual Deploy until then. Reads the same `workspaceLimits` document
 * the Usage page polls, so the two share one cache entry, but does not poll:
 * the state turns only on bex-api's five-minute sweep, and every page that
 * mounts this reads it fresh. False until the answer arrives, and on an error,
 * as Render's hook is: the server still refuses the build itself.
 */
export function usePipelineMinutesExceeded(): boolean {
  const { currentWorkspaceId } = useWorkspace();
  const { data } = useQuery(WorkspaceLimitsDocument, {
    variables: { ownerId: currentWorkspaceId ?? "" },
    skip: currentWorkspaceId == null,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  return data?.workspaceLimits?.pipelineMinutesExceeded === true;
}
