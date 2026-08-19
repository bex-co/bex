import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { GitConnectionsDocument } from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";

/** One connected GitHub account/org (ADR075). */
export interface GitConnectionRow {
  accountLogin: string;
  installationId: number;
  createdAt: string;
  /** Bare "configure grants on GitHub" deep link for this account. */
  installUrl: string;
}

/**
 * A workspace's synthesized singular connection view — `connected` is true when
 * it holds ANY connection. Kept for the "is GitHub connected at all?" callers
 * (the create-service source picker, the build & deploy section) that don't need
 * the full set. `accountLogin`/`installUrl` reflect the first (oldest) connection.
 */
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
 * The one `gitConnections` query both public hooks share (ADR075).
 * `cache-and-network` so returning from the GitHub install callback (which
 * redirects to /settings) shows the fresh connection; a backend 503 (GitHub App
 * unconfigured) arrives as `error`. Apollo normalizes on the document, so mounting
 * both hooks reads one cache entry — but the config lives here once so the two
 * cannot drift.
 */
function useGitConnectionsQuery() {
  return useQuery(GitConnectionsDocument, {
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });
}

/**
 * Collapses the connection set to the singular "is GitHub connected?" view for
 * the callers that don't need the full list. Returns `undefined` until loaded
 * (distinct from loaded-empty) so those callers can gate on definedness.
 */
export function useGitConnection(): UseGitConnectionResult {
  const { data, loading, error, refetch } = useGitConnectionsQuery();

  const connection = useMemo<GitConnectionView | undefined>(() => {
    const rows = data?.gitConnections;
    if (!rows) return undefined;
    const first = rows[0];
    return {
      connected: rows.length > 0,
      accountLogin: first?.accountLogin ?? "",
      installUrl: first?.installUrl ?? "",
    };
  }, [data]);

  return { connection, loading, error, refetch };
}

export interface UseGitConnectionsResult {
  connections: GitConnectionRow[];
  /** Whether the workspace holds any connection. */
  connected: boolean;
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
}

/**
 * Reads the full connection set for the Settings card (ADR075): every GitHub
 * account/org the workspace has connected, so the card can list them with a
 * per-account disconnect and a "Connect another account" action.
 */
export function useGitConnections(): UseGitConnectionsResult {
  const { data, loading, error, refetch } = useGitConnectionsQuery();

  const connections = useMemo<GitConnectionRow[]>(
    () =>
      (data?.gitConnections ?? [])
        .filter((c): c is NonNullable<typeof c> => c != null)
        .map((c) => ({
          accountLogin: c.accountLogin ?? "",
          installationId: c.installationId ?? 0,
          createdAt: c.createdAt ?? "",
          installUrl: c.installUrl ?? "",
        })),
    [data],
  );

  return {
    connections,
    connected: connections.length > 0,
    loading,
    error,
    refetch,
  };
}
