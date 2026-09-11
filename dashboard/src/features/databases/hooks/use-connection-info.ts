import { useCallback, useEffect, useState } from "react";
import { useApolloClient } from "@apollo/client/react";
import { DatabaseConnectionInfoDocument } from "@/graphql/definitions";
import type { ConnectionInfoView } from "@/features/databases/types";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";

export interface UseConnectionInfoResult {
  /** The revealed connection info, or null until the user asks for it. */
  info: ConnectionInfoView | null;
  loading: boolean;
  error: Error | undefined;
  /** Fetch the connection info on demand; safe to call again to refresh. */
  reveal: () => Promise<void>;
  /** Drop the revealed info from memory (re-masks the panel). */
  hide: () => void;
}

/**
 * On-demand fetch of a database's connection info + password (Render's
 * Connections panel). Deliberately NOT a `useQuery` — nothing fires on mount, so
 * the password never lands in the Apollo cache or on the wire until the user
 * clicks Reveal. `network-only` so a reveal is always fresh; `errorPolicy:none`
 * so an authz/not-provisioned error surfaces to the panel (docs/ADR006-bex-api.md:
 * connection-info requires the sensitive-read scope and 404s until CNPG has
 * generated the Secret). Confirmed access loss clears the reveal (w6/m144).
 */
export function useConnectionInfo(id: string): UseConnectionInfoResult {
  const client = useApolloClient();
  const { generation, canViewSensitive } = useCapabilities();
  const [info, setInfo] = useState<ConnectionInfoView | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | undefined>();

  useEffect(() => {
    setInfo(null);
    setError(undefined);
  }, [generation, id]);

  useEffect(() => {
    if (!canViewSensitive) {
      setInfo(null);
      setError(undefined);
    }
  }, [canViewSensitive]);

  const reveal = useCallback(async () => {
    if (!canViewSensitive) return;
    setLoading(true);
    setError(undefined);
    try {
      const res = await client.query({
        query: DatabaseConnectionInfoDocument,
        variables: { id },
        fetchPolicy: "network-only",
        errorPolicy: "none",
      });
      const ci = res.data?.databaseConnectionInfo;
      setInfo({
        password: ci?.password ?? "",
        internalConnectionString: ci?.internalConnectionString ?? "",
        externalConnectionString: ci?.externalConnectionString ?? "",
        psqlCommand: ci?.psqlCommand ?? "",
        serverCaCertificate: ci?.serverCaCertificate ?? "",
        readReplicaConnectionStrings: [],
      });
    } catch (e) {
      setError(e as Error);
    } finally {
      setLoading(false);
    }
  }, [client, id, canViewSensitive]);

  const hide = useCallback(() => {
    setInfo(null);
    setError(undefined);
  }, []);

  return { info, loading, error, reveal, hide };
}
