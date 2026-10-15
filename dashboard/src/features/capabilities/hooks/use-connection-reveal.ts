import { useCallback, useState } from "react";
import { useApolloClient } from "@apollo/client/react";
import type { ApolloClient } from "@apollo/client";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";

export interface ConnectionReveal<T> {
  /** The revealed connection info, or null until the user asks for it. */
  info: T | null;
  loading: boolean;
  error: Error | undefined;
  /** Fetch the connection info on demand; safe to call again to refresh. */
  reveal: () => Promise<void>;
  /** Drop the revealed info from memory (re-masks the panel). */
  hide: () => void;
}

interface Revealed<T> {
  id: string;
  generation: number;
  info: T | null;
  error: Error | undefined;
}

/**
 * On-demand reveal of a datastore's password-bearing connection info (Render's
 * Connections panel). Deliberately NOT a `useQuery`: nothing fires on mount, so
 * the secret never lands in the Apollo cache or on the wire until the user
 * clicks Reveal. A reveal belongs to one resource id and access generation:
 * another of either re-masks the panel, confirmed access loss drops it
 * (w6/m144), and a response that lands after either moved on is discarded.
 * `fetchInfo` must be a stable (module-level) function.
 */
export function useConnectionReveal<T>(
  id: string,
  fetchInfo: (client: ApolloClient, id: string) => Promise<T>,
): ConnectionReveal<T> {
  const client = useApolloClient();
  const { generation, canViewSensitive } = useCapabilities();
  const [revealed, setRevealed] = useState<Revealed<T>>({
    id,
    generation,
    info: null,
    error: undefined,
  });
  const [loading, setLoading] = useState(false);

  // Reset while rendering, so a secret is never painted under another
  // resource's id, not even for one frame.
  const stale =
    revealed.id !== id ||
    revealed.generation !== generation ||
    (!canViewSensitive && (revealed.info !== null || revealed.error));
  if (stale) setRevealed({ id, generation, info: null, error: undefined });
  const shown = stale ? null : revealed;

  const reveal = useCallback(async () => {
    if (!canViewSensitive) return;
    const settle = (next: Partial<Pick<Revealed<T>, "info" | "error">>) =>
      setRevealed((current) =>
        current.id === id && current.generation === generation
          ? { ...current, ...next }
          : current,
      );
    setLoading(true);
    setRevealed((current) => ({ ...current, error: undefined }));
    try {
      settle({ info: await fetchInfo(client, id) });
    } catch (e) {
      settle({ error: e as Error });
    } finally {
      setLoading(false);
    }
  }, [client, id, generation, canViewSensitive, fetchInfo]);

  const hide = useCallback(() => {
    setRevealed((current) => ({ ...current, info: null, error: undefined }));
  }, []);

  return {
    info: shown?.info ?? null,
    loading,
    error: shown?.error,
    reveal,
    hide,
  };
}
