import { useCallback, useRef } from "react";

export interface IdempotencyKey {
  /**
   * The key for the request this fingerprint stands for: the key already
   * minted when the fingerprint is the one last asked about (a retry after a
   * failure reuses it, so the server hands back what it already made), a new
   * key when it changed (an edited request is a new request).
   */
  keyFor: (fingerprint: string) => string;
  /** Forgets the key once its request succeeded. */
  release: () => void;
}

/**
 * One idempotency key per distinct submit (ADR047 D12, ADR094 D4). The
 * fingerprint, which may hold values that must not leave the browser, is kept
 * in memory only.
 */
export function useIdempotencyKey(): IdempotencyKey {
  const current = useRef<{ fingerprint: string; key: string } | null>(null);
  const keyFor = useCallback((fingerprint: string) => {
    if (current.current?.fingerprint !== fingerprint) {
      current.current = { fingerprint, key: crypto.randomUUID() };
    }
    return current.current.key;
  }, []);
  const release = useCallback(() => {
    current.current = null;
  }, []);
  return { keyFor, release };
}
