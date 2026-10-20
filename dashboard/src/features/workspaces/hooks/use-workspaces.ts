import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  WorkspacesDocument,
  type WorkspacesQuery,
} from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { useHasVerifiedSession } from "@/common/hooks/use-has-verified-session";
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
  /**
   * True when a successful membership list has arrived (including empty).
   * False while loading, skipped, or on transport/GraphQL failure — never
   * treat those as confirmed removal (w6/m144).
   */
  ready: boolean;
  /**
   * False without a session bex-api serves — none at all (a public page), or
   * one still walled behind email verification: there is no current workspace
   * then, whatever the selection cookie says (w6/078, w6/079).
   */
  verified: boolean;
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
 * firing (and 401ing) before a session exists, or while it is still walled
 * behind email verification (refused with EMAIL_VERIFICATION_REQUIRED, w6/079).
 */
export function useWorkspaces(): UseWorkspacesResult {
  const verified = useHasVerifiedSession();
  const { data, loading, error, refetch } = useQuery(WorkspacesDocument, {
    skip: !verified,
    fetchPolicy: "cache-first",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });

  const workspaces = useMemo(() => toWorkspaceViews(data?.workspaces), [data]);
  const ready = verified && !loading && data !== undefined && error == null;

  return { workspaces, loading, error, ready, verified, refetch };
}
