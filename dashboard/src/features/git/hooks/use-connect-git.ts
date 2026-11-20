import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { ClaimGitDocument, ConnectGitDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface UseConnectGitOptions {
  /**
   * Run the GitHub round trip in a NEW TAB instead of navigating the current
   * page away, for pages that hold unsaved input (w8/m31, ADR078 §3b): a
   * half-filled form survives, and the page picks the new connection up when
   * the user returns. Off for Account Settings, which holds no input and is
   * where the callback lands.
   */
  newTab?: boolean;
}

export interface UseConnectGitResult {
  /** Starts the flow and sends the browser to GitHub. */
  connect: () => Promise<void>;
  busy: boolean;
}

/**
 * Sends the browser to the GitHub URL `mint` returns for the SELECTED
 * workspace. ADR078 §6: the workspace id is required, and the action refuses to
 * fire while it is unresolved — an unscoped start binds the installation to the
 * caller's DEFAULT workspace, the live-verified wrong-tenant write.
 *
 * With `newTab`, the tab is opened SYNCHRONOUSLY inside the click handler and
 * only navigated once the URL resolves, so a popup blocker (Safari especially)
 * doesn't eat it after the await.
 */
function useGitHubFlow(
  mint: (ownerId: string) => Promise<string | null | undefined>,
  { newTab = false }: UseConnectGitOptions,
): UseConnectGitResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
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
      const url = await mint(currentWorkspaceId);
      if (!url) throw new Error("GitHub connect returned no URL");
      if (newTab) {
        if (pending) pending.location.href = url;
        else window.open(url, "_blank", "noopener,noreferrer");
        setBusy(false);
      } else {
        window.location.href = url;
      }
    } catch (err) {
      pending?.close();
      toast.error(mutationErrorMessage(err, t("git.connectError")));
      setBusy(false);
    }
  }, [mint, t, currentWorkspaceId, newTab]);

  return { connect, busy };
}

/**
 * The one "Connect GitHub account" action every starting point offers
 * (ADR078 §3b, w1/m526). It starts the OAuth claim in chooser mode — GitHub
 * OAuth is the one flow that always keeps the signed state — so the callback
 * lands on the account chooser: every installed account the user administers,
 * the connected ones marked, plus "Install on another GitHub account". An
 * account that already has the App therefore connects without reinstalling and
 * without GitHub's Configure dead end.
 */
export function useConnectGitAccount(
  options: UseConnectGitOptions = {},
): UseConnectGitResult {
  const [mutate] = useMutation(ClaimGitDocument, { fetchPolicy: "no-cache" });
  const mint = useCallback(
    async (ownerId: string) => {
      const res = await mutate({ variables: { ownerId, chooser: true } });
      return res.data?.claimGit?.claimUrl;
    },
    [mutate],
  );
  return useGitHubFlow(mint, options);
}

/**
 * The install-first flow: `connectGit` mints GitHub's stateful install URL for
 * an account that does NOT have the App yet. Only the account chooser's
 * "Install on another GitHub account" uses it — for an account that already has
 * the App, GitHub shows Configure, drops the state and never calls back
 * (ADR078 §3a), which is why no starting point offers it directly any more.
 */
export function useConnectGit(
  options: UseConnectGitOptions = {},
): UseConnectGitResult {
  const [mutate] = useMutation(ConnectGitDocument, { fetchPolicy: "no-cache" });
  const mint = useCallback(
    async (ownerId: string) => {
      const res = await mutate({ variables: { ownerId } });
      return res.data?.connectGit?.installUrl;
    },
    [mutate],
  );
  return useGitHubFlow(mint, options);
}
