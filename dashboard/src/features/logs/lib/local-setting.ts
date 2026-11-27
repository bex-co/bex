import { useSyncExternalStore } from "react";

/**
 * One browser-wide setting kept in `localStorage`, read as a hook that
 * re-renders on a write from this tab or another. Render keeps its log
 * explorer's settings this way (w3/088): one value for every explorer,
 * outliving the page.
 *
 * The server's render and the hydrating one read `fallback`, so the HTML and
 * the first client render agree, and the stored value follows straight
 * after; an explorer mounted later reads it at once. Storage that refuses a
 * write keeps the choice for this page.
 */
export function createLocalSetting<T extends string>(
  key: string,
  fallback: T,
  parse: (raw: string | null | undefined) => T,
) {
  const listeners = new Set<() => void>();
  // A choice storage refused, so it still applies until the page goes.
  let unsaved: T | null = null;

  function subscribe(listener: () => void) {
    const onStorage = (event: StorageEvent) => {
      if (event.key === key || event.key === null) listener();
    };
    listeners.add(listener);
    window.addEventListener("storage", onStorage);
    return () => {
      listeners.delete(listener);
      window.removeEventListener("storage", onStorage);
    };
  }

  function read(): T {
    if (unsaved !== null) return unsaved;
    try {
      return parse(window.localStorage.getItem(key));
    } catch {
      return fallback;
    }
  }

  function write(value: T) {
    if (read() === value) return;
    try {
      window.localStorage.setItem(key, value);
      unsaved = null;
    } catch {
      unsaved = value;
    }
    for (const listener of listeners) listener();
  }

  function useValue(): T {
    return useSyncExternalStore(subscribe, read, () => fallback);
  }

  return { useValue, read, write };
}
