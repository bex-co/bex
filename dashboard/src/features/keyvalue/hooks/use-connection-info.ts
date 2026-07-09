import { useCallback, useState } from "react";
import { useApolloClient } from "@apollo/client/react";
import { KeyValueConnectionInfoDocument } from "@/graphql/definitions";
import type { KeyValueConnectionInfoView } from "@/features/keyvalue/types";

export interface UseConnectionInfoResult {
  /** The revealed connection info, or null until the user asks for it. */
  info: KeyValueConnectionInfoView | null;
  loading: boolean;
  error: Error | undefined;
  /** Fetch the connection info on demand; safe to call again to refresh. */
  reveal: () => Promise<void>;
  /** Drop the revealed info from memory (re-masks the panel). */
  hide: () => void;
}

/**
 * On-demand fetch of a Key Value store's connection info (Render's Connections
 * panel). Deliberately NOT a `useQuery` — nothing fires on mount, so the
 * password-bearing URI never lands in the Apollo cache or on the wire until the
 * user clicks Reveal. `network-only` so a reveal is always fresh; `errorPolicy:
 * none` so an authz/not-provisioned error surfaces to the panel
 * (docs/keyvalue-management.md: connection-info 404s until the Secret exists).
 * Mirrors databases' `useConnectionInfo`.
 */
export function useConnectionInfo(id: string): UseConnectionInfoResult {
  const client = useApolloClient();
  const [info, setInfo] = useState<KeyValueConnectionInfoView | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | undefined>();

  const reveal = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const res = await client.query({
        query: KeyValueConnectionInfoDocument,
        variables: { id },
        fetchPolicy: "network-only",
        errorPolicy: "none",
      });
      const ci = res.data?.keyValueConnectionInfo;
      setInfo({
        internalConnectionString: ci?.internalConnectionString ?? "",
        externalConnectionString: ci?.externalConnectionString ?? "",
        cliCommand: ci?.cliCommand ?? "",
      });
    } catch (e) {
      setError(e as Error);
    } finally {
      setLoading(false);
    }
  }, [client, id]);

  const hide = useCallback(() => {
    setInfo(null);
    setError(undefined);
  }, []);

  return { info, loading, error, reveal, hide };
}
