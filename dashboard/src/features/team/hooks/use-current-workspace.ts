import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { WorkspacesDocument, type WorkspacesQuery } from "@/graphql/definitions";
import type { Role, WorkspaceView } from "@/features/team/types";

function toWorkspace(
  raw: NonNullable<WorkspacesQuery["workspaces"]>[number],
): WorkspaceView | null {
  if (!raw?.id) return null;
  return {
    id: raw.id,
    name: raw.name ?? raw.id,
    plan: raw.plan ?? null,
    role: (raw.role as Role | null) ?? null,
  };
}

export interface UseCurrentWorkspaceResult {
  workspace: WorkspaceView | null;
  loading: boolean;
  error: Error | undefined;
}

/**
 * Resolves the caller's current workspace from bex-api's `workspaces` — the
 * first (their primary) one. bex resolves a session caller to a single tenant
 * today (w1/m9), so the Team page manages that workspace; a full workspace
 * switcher is future work (the query already returns every membership).
 */
export function useCurrentWorkspace(): UseCurrentWorkspaceResult {
  const { data, loading, error } = useQuery(WorkspacesDocument, {
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const workspace = useMemo(() => {
    for (const raw of data?.workspaces ?? []) {
      const w = toWorkspace(raw);
      if (w) return w;
    }
    return null;
  }, [data]);

  return { workspace, loading, error };
}
