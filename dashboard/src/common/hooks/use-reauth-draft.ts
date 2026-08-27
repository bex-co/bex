import { useCallback } from "react";

/**
 * Survive-the-redirect draft persistence for unsaved form work (w3/m80 t005).
 *
 * When a session expires mid-edit, the Apollo auth link hard-navigates to login
 * (auth-redirect.ts) and the editing component unmounts — its React state, and
 * with it the user's in-progress edits, would be gone. WCAG 2.2.5 requires that
 * re-authentication not cost the user their work, so a caller mirrors its
 * working value into `sessionStorage` on every change; after sign-in returns to
 * the same page, `consumeRestored()` hands it back.
 *
 * `sessionStorage` (not `localStorage`) is deliberate: per-tab, cleared when the
 * tab closes, never synced or sent to a server. A short TTL bounds how long a
 * draft lingers, and the caller clears it on a successful save. Only user-typed
 * working values ever reach here — never a secret fetched from the API purely to
 * display it (those are revealed on demand and never enter the draft).
 */
const PREFIX = "bex:reauth-draft:";
const DEFAULT_TTL_MS = 60 * 60 * 1000; // 1h

interface StoredDraft<T> {
  at: number;
  value: T;
}

export interface ReauthDraft<T> {
  /** Persist the current working value. Call on every change. */
  save: (value: T) => void;
  /** Drop the persisted draft. Call on a successful save or explicit discard. */
  clear: () => void;
  /**
   * Read and DELETE the persisted draft, or null if none survived within the
   * TTL. Call once from a mount effect (client-only) so the restore doesn't
   * diverge from the SSR render — reading in a render initializer would hydrate
   * an edit state the server never produced.
   */
  consumeRestored: () => T | null;
}

export function useReauthDraft<T>(
  key: string,
  ttlMs: number = DEFAULT_TTL_MS,
): ReauthDraft<T> {
  const storageKey = PREFIX + key;

  const save = useCallback(
    (value: T) => {
      if (typeof window === "undefined") return;
      try {
        const payload: StoredDraft<T> = { at: Date.now(), value };
        window.sessionStorage.setItem(storageKey, JSON.stringify(payload));
      } catch {
        // Storage full/blocked (private mode, quota) — losing the safety net is
        // acceptable; the edit itself is unaffected.
      }
    },
    [storageKey],
  );

  const clear = useCallback(() => {
    if (typeof window === "undefined") return;
    try {
      window.sessionStorage.removeItem(storageKey);
    } catch {
      // ignore
    }
  }, [storageKey]);

  const consumeRestored = useCallback((): T | null => {
    if (typeof window === "undefined") return null;
    let raw: string | null = null;
    try {
      raw = window.sessionStorage.getItem(storageKey);
      if (raw !== null) window.sessionStorage.removeItem(storageKey);
    } catch {
      return null;
    }
    if (!raw) return null;
    try {
      const parsed = JSON.parse(raw) as StoredDraft<T>;
      if (
        !parsed ||
        typeof parsed.at !== "number" ||
        Date.now() - parsed.at > ttlMs
      ) {
        return null;
      }
      return parsed.value;
    } catch {
      return null;
    }
  }, [storageKey, ttlMs]);

  return { save, clear, consumeRestored };
}
