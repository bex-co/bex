import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  WorkspacesDocument,
  type WorkspacesQuery,
} from "@/graphql/definitions";
import { useIsAuthenticated } from "@/common/hooks/use-is-authenticated";
import type { WorkspaceView } from "@/features/workspaces/types";

type RawWorkspace = NonNullable<WorkspacesQuery["workspaces"]>[number];

function toWorkspaceViews(
  raw: WorkspacesQuery["workspaces"] | undefined,
): WorkspaceView[] {
  return (raw ?? [])
    .filter((w): w is NonNullable<RawWorkspace> & { id: string } => !!w?.id)
    .map((w) => ({
      id: w.id,
      name: w.name ?? "",
      plan: w.plan ?? "hobby",
      role: w.role ?? "",
      createdAt: w.createdAt,
    }));
}

export interface UseWorkspacesResult {
  workspaces: WorkspaceView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the list query (callers refresh after create/rename/delete). */
  refetch: () => Promise<unknown>;
}

/**
 * Reads bex-api's `workspaces` — the caller's memberships (w6/m1), source of
 * the switcher's list and the `/new/workspace` Hobby-cap check. Membership-
 * scoped server-side, so this is never more than "workspaces I belong to."
 * WorkspaceProvider mounts at the app root (so hooks called before a page
 * returns `<DashboardLayout>` still see it), which puts public pages
 * (login/sign-up/...) in its subtree too — `skip` keeps this query from
 * firing (and 401ing) before a session exists.
 */
export function useWorkspaces(): UseWorkspacesResult {
  const authenticated = useIsAuthenticated();
  const { data, loading, error, refetch } = useQuery(WorkspacesDocument, {
    skip: !authenticated,
    fetchPolicy: "cache-first",
    errorPolicy: "all",
  });

  const workspaces = useMemo(() => toWorkspaceViews(data?.workspaces), [data]);

  return { workspaces, loading, error, refetch };
}
