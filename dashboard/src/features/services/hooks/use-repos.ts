import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { ReposDocument } from "@/graphql/definitions";
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
}

export function useRepos(): UseReposResult {
  // ADR075 §6: the repo list is the SELECTED workspace's connection set, never
  // the caller's default one; defer while the workspace id resolves.
  const { currentWorkspaceId } = useWorkspace();
  const { data, loading, error } = useQuery(ReposDocument, {
    variables: { ownerId: currentWorkspaceId },
    skip: currentWorkspaceId == null,
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

  return { repos, loading, error };
}
