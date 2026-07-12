import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DisconnectGitDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseDisconnectGitResult {
  /** Fires disconnectGit; resolves true on success (toasted either way). */
  disconnect: () => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Disconnect action to bex-api's `disconnectGit`. On success the
 * workspace's repos empty and push-to-deploy via the app stops; the manual HMAC
 * webhook path is unaffected.
 */
export function useDisconnectGit(): UseDisconnectGitResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(DisconnectGitDocument);
  const [busy, setBusy] = useState(false);

  const disconnect = useCallback(async () => {
    setBusy(true);
    try {
      await mutate();
      toast.success(t("git.disconnectSuccess"));
      return true;
    } catch {
      toast.error(t("git.disconnectError"));
      return false;
    } finally {
      setBusy(false);
    }
  }, [mutate, t]);

  return { disconnect, busy };
}
