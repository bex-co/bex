import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { ConnectGitDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface UseConnectGitResult {
  /** Fires connectGit and navigates the browser to the GitHub install URL. */
  connect: () => Promise<void>;
  busy: boolean;
}

/**
 * Wires the "Connect GitHub" button to bex-api's `connectGit`, then sends the
 * browser to GitHub's install screen (installUrl). GitHub's post-install
 * callback (backend GET /v1/git/callback) records the installation and redirects
 * back to /settings, where useGitConnection's refetch shows it connected.
 *
 * ADR075 §6: the mutation carries the SELECTED workspace's ownerId and refuses
 * to fire while it is unresolved — an unscoped connect binds the installation to
 * the caller's DEFAULT workspace, the live-verified wrong-tenant write.
 */
export function useConnectGit(): UseConnectGitResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(ConnectGitDocument, { fetchPolicy: "no-cache" });
  const [busy, setBusy] = useState(false);

  const connect = useCallback(async () => {
    if (currentWorkspaceId == null) {
      toast.error(t("git.connectError"));
      return;
    }
    setBusy(true);
    try {
      const res = await mutate({
        variables: { ownerId: currentWorkspaceId },
      });
      const url = res.data?.connectGit?.installUrl;
      if (!url) throw new Error("connectGit returned no install URL");
      window.location.href = url;
    } catch {
      toast.error(t("git.connectError"));
      setBusy(false);
    }
  }, [mutate, t, currentWorkspaceId]);

  return { connect, busy };
}
