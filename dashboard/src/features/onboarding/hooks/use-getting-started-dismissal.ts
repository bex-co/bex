import { useCallback, useSyncExternalStore } from "react";

// ADR075 D2: dismissal persists client-side only — no backend state. Keyed by
// workspace so each workspace's card is dismissed on its own.
const storageKey = (workspaceId: string) =>
  `bex.gettingStarted.dismissed.${workspaceId}`;

const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
  listeners.add(listener);
  window.addEventListener("storage", listener);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", listener);
  };
}

function readDismissed(workspaceId: string | null): boolean {
  if (!workspaceId) return true;
  try {
    return window.localStorage.getItem(storageKey(workspaceId)) === "1";
  } catch {
    return false; // storage blocked: the card stays dismissible per session
  }
}

/**
 * `dismissed` is `true` on the server and before a workspace is known, so a
 * card the user already dismissed never flashes during hydration.
 */
export function useGettingStartedDismissal(workspaceId: string | null) {
  const dismissed = useSyncExternalStore(
    subscribe,
    () => readDismissed(workspaceId),
    () => true,
  );
  const dismiss = useCallback(() => {
    if (!workspaceId) return;
    try {
      window.localStorage.setItem(storageKey(workspaceId), "1");
    } catch {
      // storage blocked: nothing to persist
    }
    for (const listener of listeners) listener();
  }, [workspaceId]);
  return { dismissed, dismiss };
}
