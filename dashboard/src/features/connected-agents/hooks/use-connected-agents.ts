import { useCallback, useState } from "react";
import { toast } from "sonner";
import { useTranslations } from "@/common/hooks/use-translations";
import { useFetchedList } from "@/common/hooks/use-fetched-list";
import type { ConnectedAgentView } from "@/features/connected-agents/types";

export interface UseConnectedAgentsResult {
  agents: ConnectedAgentView[];
  loading: boolean;
  error: boolean;
  /** Fires the revoke call; resolves true on success (toasted either way). */
  revoke: (clientId: string, clientName: string) => Promise<boolean>;
  /** The clientId currently being revoked, or null (disables that row's control). */
  revoking: string | null;
  refetch: () => void;
}

/**
 * Reads `/api/connected-agents` — the dashboard SSR endpoint over Hydra's admin
 * consent-session API (w4/m18). Not GraphQL: Hydra is a dashboard-only
 * dependency, so this is a plain fetch against the dashboard's own server-fn
 * route, not bex-api. A successful revoke removes the row via `setData`
 * immediately (unlike `useRevokeApiKey`, there's no server-side list to
 * reconcile against — Hydra has just invalidated every one of that client's
 * tokens, so the optimistic filter already matches server truth).
 */
export function useConnectedAgents(): UseConnectedAgentsResult {
  const { t } = useTranslations();
  const {
    data: agents,
    loading,
    error,
    setData: setAgents,
    refetch,
  } = useFetchedList<ConnectedAgentView>("/api/connected-agents");
  const [revoking, setRevoking] = useState<string | null>(null);

  const revoke = useCallback(
    async (clientId: string, clientName: string) => {
      setRevoking(clientId);
      try {
        const res = await fetch("/api/connected-agents", {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ clientId }),
        });
        if (!res.ok) throw new Error("revoke failed");
        setAgents((prev) => prev.filter((a) => a.clientId !== clientId));
        toast.success(t("connectedAgents.revokeSuccess", { name: clientName }));
        return true;
      } catch {
        toast.error(t("connectedAgents.revokeError", { name: clientName }));
        return false;
      } finally {
        setRevoking(null);
      }
    },
    [setAgents, t],
  );

  return { agents, loading, error, revoke, revoking, refetch };
}
