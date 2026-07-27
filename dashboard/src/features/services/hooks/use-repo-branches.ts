import { useQuery } from "@apollo/client/react";
import { RepoBranchesDocument } from "@/graphql/definitions";

export interface UseRepoBranchesResult {
  branches: string[];
  loading: boolean;
}

/**
 * Fetches the actual branches of a connected GitHub repo (w5/m54), feeding the
 * Settings Branch combobox. Returns an empty list — never throws — for a
 * non-GitHub repo, no GitHub App connection, or a backend error, so the Branch
 * row degrades to free-text entry. Skips the query entirely when repo is empty.
 */
export function useRepoBranches(
  repo: string | null | undefined,
): UseRepoBranchesResult {
  const { data, loading } = useQuery(RepoBranchesDocument, {
    variables: { repo: repo ?? "" },
    skip: !repo,
    // A branch list is best-effort; an error must never block editing the field.
    errorPolicy: "all",
  });
  const branches = (data?.repoBranches ?? []).filter((b): b is string => !!b);
  return { branches, loading };
}
