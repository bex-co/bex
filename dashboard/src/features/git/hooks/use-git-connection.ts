import { useQuery } from "@apollo/client/react";
import { GitConnectionDocument } from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";

export interface GitConnectionView {
  connected: boolean;
  accountLogin: string;
  installUrl: string;
}

export interface UseGitConnectionResult {
  connection: GitConnectionView | undefined;
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
}

/**
 * Reads bex-api's `gitConnection` — whether this workspace has connected a
 * GitHub App installation (docs/github-integration.md). `cache-and-network` so
 * returning from the GitHub install callback (which redirects to /settings)
 * shows the fresh connection; the card also refetches on window focus. A backend
 * 503 (GitHub App unconfigured) arrives as `error`, which the card renders as
 * "unavailable".
 */
export function useGitConnection(): UseGitConnectionResult {
  const { data, loading, error, refetch } = useQuery(GitConnectionDocument, {
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });

  const raw = data?.gitConnection;
  const connection: GitConnectionView | undefined = raw
    ? {
        connected: !!raw.connected,
        accountLogin: raw.accountLogin ?? "",
        installUrl: raw.installUrl ?? "",
      }
    : undefined;

  return { connection, loading, error, refetch };
}
