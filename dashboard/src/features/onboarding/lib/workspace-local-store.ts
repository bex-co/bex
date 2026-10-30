import { useSyncExternalStore } from "react";

/**
 * A small per-workspace value kept in localStorage only (ADR075: no backend
 * onboarding state), readable as a hook that re-renders on writes from this
 * tab or another one. Blocked storage reads as `fallback` and writes nothing.
 */
export function createWorkspaceLocalStore<T extends string>(
  prefix: string,
  fallback: T,
  parse: (raw: string | null) => T,
) {
  const key = (workspaceId: string) => `${prefix}.${workspaceId}`;
  const listeners = new Set<() => void>();

  function subscribe(listener: () => void) {
    listeners.add(listener);
    window.addEventListener("storage", listener);
    return () => {
      listeners.delete(listener);
      window.removeEventListener("storage", listener);
    };
  }

  function read(workspaceId: string | null): T {
    if (!workspaceId) return fallback;
    try {
      return parse(window.localStorage.getItem(key(workspaceId)));
    } catch {
      return fallback;
    }
  }

  function write(workspaceId: string | null, value: T) {
    if (!workspaceId || read(workspaceId) === value) return;
    try {
      window.localStorage.setItem(key(workspaceId), value);
    } catch {
      return;
    }
    for (const listener of listeners) listener();
  }

  /** `serverValue` is what SSR and hydration render before storage is read. */
  function useValue(workspaceId: string | null, serverValue: T = fallback): T {
    return useSyncExternalStore(
      subscribe,
      () => read(workspaceId),
      () => serverValue,
    );
  }

  return { useValue, write };
}
