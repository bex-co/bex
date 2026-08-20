import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { ClaimGitDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface UseClaimGitResult {
  /**
   * Fires claimGit (ADR075 §3a) and navigates to GitHub's OAuth authorize
   * screen — the flow that binds an installation ALREADY present on GitHub,
   * where the install URL would strip the signed state.
   */
  claim: () => Promise<void>;
  busy: boolean;
}

/**
 * Wires the "Claim installed account" action to bex-api's `claimGit`. The
 * callback resolves the sole unbound installation the authorizing user
 * administers and binds it to the SELECTED workspace (ownerId threaded per
 * ADR075 §6, refusing while unresolved).
 */
export function useClaimGit(): UseClaimGitResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(ClaimGitDocument, { fetchPolicy: "no-cache" });
  const [busy, setBusy] = useState(false);

  const claim = useCallback(async () => {
    if (currentWorkspaceId == null) {
      toast.error(t("git.claimError"));
      return;
    }
    setBusy(true);
    try {
      const res = await mutate({
        variables: { ownerId: currentWorkspaceId },
      });
      const url = res.data?.claimGit?.claimUrl;
      if (!url) throw new Error("claimGit returned no claim URL");
      window.location.href = url;
    } catch {
      toast.error(t("git.claimError"));
      setBusy(false);
    }
  }, [mutate, t, currentWorkspaceId]);

  return { claim, busy };
}
