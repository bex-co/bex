import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { ConnectGitDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface UseConnectGitOptions {
  /**
   * Open GitHub's install screen in a NEW TAB instead of navigating the current
   * page away. Used by the in-place credentials menu on the create-service /
   * Blueprint / Update-Source source picker (w8/m31) so a half-filled create
   * form survives the round trip — the caller refetches connections on focus
   * when the user returns. The Settings card leaves this off (a dedicated page
   * where full navigation is fine).
   */
  newTab?: boolean;
}

export interface UseConnectGitResult {
  /** Fires connectGit and sends the browser to the GitHub install URL. */
  connect: () => Promise<void>;
  busy: boolean;
}

/**
 * Wires the "Connect GitHub" button to bex-api's `connectGit`, then sends the
 * browser to GitHub's install screen (installUrl). GitHub's post-install
 * callback (backend GET /v1/git/callback) records the installation and redirects
 * back to /settings, where useGitConnection's refetch shows it connected.
 *
 * With `newTab`, the install opens in a new tab so the current page (e.g. a
 * half-filled /services/new form) is preserved; the tab is opened SYNCHRONOUSLY
 * inside the click handler and only navigated once `installUrl` resolves, so a
 * popup blocker (Safari especially) doesn't eat it after the await.
 *
 * ADR075 §6: the mutation carries the SELECTED workspace's ownerId and refuses
 * to fire while it is unresolved — an unscoped connect binds the installation to
 * the caller's DEFAULT workspace, the live-verified wrong-tenant write.
 */
export function useConnectGit(
  options: UseConnectGitOptions = {},
): UseConnectGitResult {
  const { newTab = false } = options;
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(ConnectGitDocument, { fetchPolicy: "no-cache" });
  const [busy, setBusy] = useState(false);

  const connect = useCallback(async () => {
    if (currentWorkspaceId == null) {
      toast.error(t("git.connectError"));
      return;
    }
    // Pre-open a blank tab in the click's gesture context so the later
    // navigation isn't blocked; stays null (and unused) in same-tab mode.
    const pending = newTab ? window.open("", "_blank") : null;
    setBusy(true);
    try {
      const res = await mutate({
        variables: { ownerId: currentWorkspaceId },
      });
      const url = res.data?.connectGit?.installUrl;
      if (!url) throw new Error("connectGit returned no install URL");
      if (newTab) {
        if (pending) pending.location.href = url;
        else window.open(url, "_blank", "noopener,noreferrer");
        setBusy(false);
      } else {
        window.location.href = url;
      }
    } catch {
      pending?.close();
      toast.error(t("git.connectError"));
      setBusy(false);
    }
  }, [mutate, t, currentWorkspaceId, newTab]);

  return { connect, busy };
}
