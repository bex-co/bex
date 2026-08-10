import { useContext, useEffect, type ReactNode } from "react";
import { getApolloContext } from "@apollo/client/react";
import { trackEvent } from "@/common/lib/telemetry";
import { useHasSSHKey } from "@/features/ssh-keys/hooks/use-has-ssh-key";

export interface RequiresSshKeyProps {
  /** The real SSH affordance (Open in Zed, `ssh …` command, …). */
  children: ReactNode;
  /** Shown ONLY when the caller is confirmed to have zero registered keys. */
  fallback: ReactNode;
  /** Bounded label for the activation funnel (e.g. "agent-session-zed"). */
  surface?: string;
}

/**
 * Gates an SSH affordance behind key registration (w2/m66). It swaps the raw,
 * doomed payload (`ssh …` / `zed://…`) for a `fallback` CTA ONLY when the caller
 * is confirmed to have no key — the dominant first-run failure the author hit
 * live (`Permission denied (publickey)`, gateway metric `rejected_key`).
 *
 * Fail-open: while the shared key query is loading OR errored — OR when there is
 * no Apollo client at all to ask (an isolated render / SSR without a client) —
 * the real affordance renders. A guard must never hide a working feature on its
 * own trouble; an over-eager gate is worse than no gate. The gate is an
 * activation *nudge*, not a correctness guarantee: having ≥1 key doesn't prove
 * the specific key the local `ssh` will offer is registered, so it reduces but
 * can't eliminate failures.
 *
 * It swaps the payload, never the entrypoint — callers keep the trigger visible
 * so the feature stays discoverable.
 */
export function RequiresSshKey(props: RequiresSshKeyProps) {
  // Detect the client without throwing (useQuery would). No client => fail open.
  const { client } = useContext(getApolloContext());
  if (!client) return <>{props.children}</>;
  return <GateWithClient {...props} />;
}

function GateWithClient({ children, fallback, surface }: RequiresSshKeyProps) {
  const { hasKey, loading, error } = useHasSSHKey();
  const showFallback = !hasKey && !loading && !error;

  useEffect(() => {
    if (showFallback) {
      trackEvent("ssh_gate_shown", surface ? { surface } : undefined);
    }
  }, [showFallback, surface]);

  if (hasKey || loading || error) return <>{children}</>;
  return <>{fallback}</>;
}
