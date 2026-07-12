import { useQuery } from "@apollo/client/react";
import { GitConnectionDocument } from "@/graphql/definitions";

export interface GitConnectionView {
  connected: boolean;
  accountLogin: string;
  installUrl: string;
}

export interface UseGitConnectionResult {
  connection: GitConnectionView | null;
  loading: boolean;
}

export function useGitConnection(): UseGitConnectionResult {
  const { data, loading } = useQuery(GitConnectionDocument);

  const raw = data?.gitConnection;
  const connection: GitConnectionView | null = raw
    ? {
        connected: raw.connected ?? false,
        accountLogin: raw.accountLogin ?? "",
        installUrl: raw.installUrl ?? "",
      }
    : null;

  return { connection, loading };
}
