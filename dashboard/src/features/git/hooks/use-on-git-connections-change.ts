import { useEffect, useRef } from "react";
import { useGitConnections } from "@/features/git/hooks/use-git-connection";

/**
 * Calls `onChange` whenever the SET of connected installations changes after
 * it first loaded — an account connected in another tab, or one disconnected
 * here (w1/m526). The connections query polls, so this fires without a focus
 * event: the repo list follows the connection set instead of waiting for the
 * user to refocus the window, which a new-tab connect does not always raise.
 *
 * `poll: false` when the page already polls the connections elsewhere (the
 * source picker does), so this watcher adds no timer of its own.
 */
export function useOnGitConnectionsChange(
  onChange: () => void,
  { poll = true }: { poll?: boolean } = {},
): void {
  const { connections, loading } = useGitConnections({ poll });
  const key = connections
    .map((c) => c.installationId)
    .sort((a, b) => a - b)
    .join(",");
  const seen = useRef<string | null>(null);
  useEffect(() => {
    if (loading) return;
    if (seen.current !== null && seen.current !== key) onChange();
    seen.current = key;
  }, [key, loading, onChange]);
}
