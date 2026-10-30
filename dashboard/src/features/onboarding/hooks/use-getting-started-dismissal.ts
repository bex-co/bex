import { useCallback } from "react";
import { createWorkspaceLocalStore } from "../lib/workspace-local-store";

// ADR075 D2: dismissal persists client-side only — no backend state. Keyed by
// workspace so each workspace's card is dismissed on its own. Blocked storage
// keeps the card dismissible per session.
const store = createWorkspaceLocalStore<"0" | "1">(
  "bex.gettingStarted.dismissed",
  "0",
  (raw) => (raw === "1" ? "1" : "0"),
);

/**
 * `dismissed` is `true` on the server and before a workspace is known, so a
 * card the user already dismissed never flashes during hydration.
 */
export function useGettingStartedDismissal(workspaceId: string | null) {
  const value = store.useValue(workspaceId, "1");
  const dismissed = !workspaceId || value === "1";
  const dismiss = useCallback(
    () => store.write(workspaceId, "1"),
    [workspaceId],
  );
  return { dismissed, dismiss };
}
