import { useCallback, useSyncExternalStore } from "react";
import {
  parseEnvironmentResourceFilter,
  type EnvironmentResourceFilter,
} from "@/features/projects/lib/resource-filter";

/**
 * Each environment section's All / Services / Env Groups choice, kept per
 * environment in `localStorage` as Render keeps it (w1/m682): one JSON object
 * from environment id to choice, read safely, so a malformed value or a
 * refused read falls back to All.
 *
 * The server's render and the hydrating one read All, so the HTML and the
 * first client render agree, and the stored choice follows straight after.
 * Storage that refuses a write keeps the choice for this page.
 */
export const SECTION_FILTER_STORAGE_KEY = "bex-project-environment-filters";

type FilterMap = Record<string, unknown>;

const listeners = new Set<() => void>();
// Choices storage refused, so they still apply until the page goes.
const unsaved = new Map<string, EnvironmentResourceFilter>();

function readRaw(): string | null {
  try {
    return window.localStorage.getItem(SECTION_FILTER_STORAGE_KEY) ?? null;
  } catch {
    return null;
  }
}

function parseMap(raw: string | null): FilterMap {
  if (!raw) return {};
  try {
    const value: unknown = JSON.parse(raw);
    return typeof value === "object" && value !== null && !Array.isArray(value)
      ? (value as FilterMap)
      : {};
  } catch {
    return {};
  }
}

function subscribe(listener: () => void) {
  const onStorage = (event: StorageEvent) => {
    if (event.key === SECTION_FILTER_STORAGE_KEY || event.key === null) {
      listener();
    }
  };
  listeners.add(listener);
  window.addEventListener("storage", onStorage);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", onStorage);
  };
}

function read(environmentId: string): EnvironmentResourceFilter {
  return (
    unsaved.get(environmentId) ??
    parseEnvironmentResourceFilter(parseMap(readRaw())[environmentId])
  );
}

function write(environmentId: string, filter: EnvironmentResourceFilter) {
  const map = parseMap(readRaw());
  map[environmentId] = filter;
  try {
    window.localStorage.setItem(
      SECTION_FILTER_STORAGE_KEY,
      JSON.stringify(map),
    );
    unsaved.delete(environmentId);
  } catch {
    unsaved.set(environmentId, filter);
  }
  for (const listener of listeners) listener();
}

/** One section's remembered choice and its setter. */
export function useSectionFilter(
  environmentId: string,
): [EnvironmentResourceFilter, (filter: EnvironmentResourceFilter) => void] {
  const filter = useSyncExternalStore(
    subscribe,
    () => read(environmentId),
    () => "all" as const,
  );
  const setFilter = useCallback(
    (next: EnvironmentResourceFilter) => write(environmentId, next),
    [environmentId],
  );
  return [filter, setFilter];
}
