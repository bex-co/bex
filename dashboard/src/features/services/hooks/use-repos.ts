import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { ReposDocument } from "@/graphql/definitions";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface RepoView {
  id: number;
  fullName: string;
  private: boolean;
  defaultBranch: string;
  htmlUrl: string;
  cloneUrl: string;
  /** The GitHub account/org this repo belongs to — the picker groups by it (ADR075). */
  accountLogin: string;
}

export interface UseReposResult {
  repos: RepoView[];
  loading: boolean;
  error: Error | undefined;
  /**
   * Re-runs the repos query. The credentials menu / source picker call it on
   * window focus so a newly-connected account's repos appear in place after the
   * new-tab install returns (w8/m31); Apollo normalizes on the document, so the
   * refreshed list updates every `useRepos` reader.
   */
  refetch: () => Promise<unknown>;
}

export function useRepos(): UseReposResult {
  // ADR075 §6: the repo list is the SELECTED workspace's connection set, never
  // the caller's default one; defer while the workspace id resolves.
  const { currentWorkspaceId } = useWorkspace();
  // bex-api lists repositories for the roles that can use one (developer and
  // up). Another role's request is refused and recorded as a denial, so it is
  // never sent: a viewer's picker is an empty list behind a disabled form.
  const {
    canCreate,
    loading: capabilitiesLoading,
    loaded: capabilitiesLoaded,
  } = useCapabilities();
  const { data, loading, error, refetch } = useQuery(ReposDocument, {
    variables: { ownerId: currentWorkspaceId },
    skip: currentWorkspaceId == null || !canCreate,
  });

  const repos = useMemo<RepoView[]>(
    () =>
      (data?.repos ?? [])
        .filter((r): r is NonNullable<typeof r> => r?.fullName != null)
        .map((r) => ({
          id: r.id ?? 0,
          fullName: r.fullName!,
          private: r.private ?? false,
          defaultBranch: r.defaultBranch ?? "main",
          htmlUrl: r.htmlUrl ?? "",
          cloneUrl: r.cloneUrl ?? "",
          accountLogin: r.accountLogin ?? "",
        })),
    [data],
  );

  // The role is still being read: the list is pending, not empty.
  const awaitingRole = !canCreate && capabilitiesLoading && !capabilitiesLoaded;
  return { repos, loading: loading || awaitingRole, error, refetch };
}
