import { createContext, useContext } from "react";
import type { KeyValueView } from "@/features/keyvalue/types";

/**
 * What a Key Value instance's layout route hands each of its pages (w3/m649):
 * the instance it read and the re-read that converges its status after a
 * change. The layout mounts its `<Outlet/>` only once the instance is loaded
 * and its workspace is the selection, so a page never renders without one.
 */
export interface KeyValueDetail {
  keyValue: KeyValueView;
  refetch: () => void;
}

const KeyValueDetailContext = createContext<KeyValueDetail | null>(null);

export const KeyValueDetailProvider = KeyValueDetailContext.Provider;

/** The open instance, for a page under `/keyvalue/$keyValueId`. */
export function useKeyValueDetail(): KeyValueDetail {
  const detail = useContext(KeyValueDetailContext);
  if (!detail) {
    throw new Error("useKeyValueDetail outside the Key Value layout");
  }
  return detail;
}
