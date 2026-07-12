import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { ReposDocument } from "@/graphql/definitions";

export interface RepoView {
  id: number;
  fullName: string;
  private: boolean;
  defaultBranch: string;
  htmlUrl: string;
  cloneUrl: string;
}

export interface UseReposResult {
  repos: RepoView[];
  loading: boolean;
  error: Error | undefined;
}

export function useRepos(): UseReposResult {
  const { data, loading, error } = useQuery(ReposDocument);

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
        })),
    [data],
  );

  return { repos, loading, error };
}
