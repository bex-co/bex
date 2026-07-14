import { useCallback, useState } from "react";
import { toast } from "sonner";
import { useTranslations } from "@/common/hooks/use-translations";
import { useFetchedList } from "@/common/hooks/use-fetched-list";
import type { SessionView } from "@/features/sessions/types";

export interface UseActiveSessionsResult {
  sessions: SessionView[];
  loading: boolean;
  error: boolean;
  /** Revoke one specific other session. Resolves true on success (toasted either way). */
  revoke: (id: string) => Promise<boolean>;
  /** The session id currently being revoked, or null. */
  revoking: string | null;
  /** Sign out every session but the current one. */
  signOutOthers: () => Promise<boolean>;
  signingOutOthers: boolean;
  refetch: () => void;
}

/**
 * Reads `/api/sessions` — the dashboard SSR endpoint over Kratos's self-service
 * FrontendApi (w4/006 folded into w4/m18). A plain fetch, not GraphQL: session
 * management is Kratos-owned, not a bex-api concern. A successful revoke or
 * sign-out-others updates the list via `setData` immediately — the optimistic
 * filter already matches server truth, since Kratos has just invalidated
 * exactly those sessions.
 */
export function useActiveSessions(): UseActiveSessionsResult {
  const { t } = useTranslations();
  const {
    data: sessions,
    loading,
    error,
    setData: setSessions,
    refetch,
  } = useFetchedList<SessionView>("/api/sessions");
  const [revoking, setRevoking] = useState<string | null>(null);
  const [signingOutOthers, setSigningOutOthers] = useState(false);

  const revoke = useCallback(
    async (id: string) => {
      setRevoking(id);
      try {
        const res = await fetch("/api/sessions", {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ action: "revoke", id }),
        });
        if (!res.ok) throw new Error("revoke failed");
        setSessions((prev) => prev.filter((s) => s.id !== id));
        toast.success(t("activeSessions.revokeSuccess"));
        return true;
      } catch {
        toast.error(t("activeSessions.revokeError"));
        return false;
      } finally {
        setRevoking(null);
      }
    },
    [setSessions, t],
  );

  const signOutOthers = useCallback(async () => {
    setSigningOutOthers(true);
    try {
      const res = await fetch("/api/sessions", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ action: "sign-out-others" }),
      });
      if (!res.ok) throw new Error("sign-out-others failed");
      setSessions((prev) => prev.filter((s) => s.current));
      toast.success(t("activeSessions.signOutOthersSuccess"));
      return true;
    } catch {
      toast.error(t("activeSessions.signOutOthersError"));
      return false;
    } finally {
      setSigningOutOthers(false);
    }
  }, [setSessions, t]);

  return {
    sessions,
    loading,
    error,
    revoke,
    revoking,
    signOutOthers,
    signingOutOthers,
    refetch,
  };
}
