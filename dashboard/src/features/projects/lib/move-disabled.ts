import { createContext, useContext } from "react";

/**
 * Render's `disableMove` (w3/m680): a resource list whose rows are not moved
 * from it, a Blueprint's Resources page, provides `true`, and each row menu's
 * Move to project and the selection toolbar's Move leave that list.
 */
export const MoveDisabledContext = createContext(false);

export function useMoveDisabled(): boolean {
  return useContext(MoveDisabledContext);
}
