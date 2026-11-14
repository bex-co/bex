import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { WorkspaceIpAllowListDocument } from "@/graphql/definitions";
import {
  toIPAllowListDrafts,
  type IPAllowListEntryDraft,
} from "@/common/lib/ip-allow-list";

export interface UseWorkspaceIPAllowListResult {
  /**
   * The workspace's saved inbound IP rules in the editor's shape (Render's
   * seeded allow-all pair when nobody saved any; empty is deny-all), or null
   * while unread or when the read failed.
   */
  entries: IPAllowListEntryDraft[] | null;
  /** The first read is in flight and there is nothing to show yet. */
  loading: boolean;
  error: Error | undefined;
}

/**
 * Reads one workspace's inbound IP rules for the Workspace Settings card
 * (w9/m388) through bex-api's `workspace(id)`, Render's owner read. Never
 * polled, and never part of the polled `Workspaces` list: bex-api resolves the
 * field only when selected. `setWorkspaceIPAllowList` answers the same
 * normalized Workspace, so a save re-renders the card without a refetch.
 */
export function useWorkspaceIPAllowList(
  workspaceId: string,
): UseWorkspaceIPAllowListResult {
  const { data, loading, error } = useQuery(WorkspaceIpAllowListDocument, {
    variables: { id: workspaceId },
    errorPolicy: "all",
  });
  const entries = useMemo(
    () => toIPAllowListDrafts(data?.workspace?.ipAllowList),
    [data],
  );
  return { entries, loading: loading && data === undefined, error };
}
