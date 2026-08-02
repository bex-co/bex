import { createContext, useContext, type ReactNode } from "react";
import { useCurrentUser, type UseCurrentUser } from "./use-current-user";

const CurrentUserContext = createContext<UseCurrentUser | null>(null);

/**
 * Runs the personal-status read once for the whole authenticated app so the
 * drawer footer reads it from context instead of refetching every time the
 * drawer (and its footer) mounts on open. Mounted alongside the drawer, above
 * the visibility gate, so the read is keyed to the identity — not to how often
 * the menu is opened.
 */
export function CurrentUserProvider({ children }: { children: ReactNode }) {
  const value = useCurrentUser();
  return (
    <CurrentUserContext.Provider value={value}>
      {children}
    </CurrentUserContext.Provider>
  );
}

export function useCurrentUserState(): UseCurrentUser {
  const value = useContext(CurrentUserContext);
  if (!value) {
    throw new Error(
      "useCurrentUserState must be used inside a CurrentUserProvider",
    );
  }
  return value;
}
