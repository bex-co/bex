import { useCallback, useEffect, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { AcceptWorkspaceInviteDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  peekPendingInviteToken,
  retainPendingInviteToken,
  takePendingInviteToken,
} from "@/common/lib/invite-token";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { classifyInviteRedemptionError } from "./invite-redemption-error";

/**
 * Offers workspace-invite acceptance after the caller is authenticated
 * (w1/m33 + codex round-16 #8). The token comes from the emailed link
 * (`?invite=` or the sessionStorage stash written before the Kratos
 * round-trip). Navigation alone never mutates membership — the user must
 * click Accept. Decline clears the pending capability.
 */
export function useInviteRedemption() {
  const { t } = useTranslations();
  const { refetch } = useWorkspace();
  const [acceptMut] = useMutation(AcceptWorkspaceInviteDocument);
  const [pendingToken, setPendingToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined") return;
    setPendingToken(peekPendingInviteToken());
  }, []);

  const decline = useCallback(() => {
    takePendingInviteToken();
    setPendingToken(null);
  }, []);

  const accept = useCallback(async () => {
    if (busy) return;
    const token = takePendingInviteToken();
    setPendingToken(null);
    if (!token) return;
    setBusy(true);
    try {
      let data;
      try {
        ({ data } = await acceptMut({ variables: { token } }));
      } catch (e) {
        const failure = classifyInviteRedemptionError(e);
        if (failure === "ambiguous") {
          retainPendingInviteToken(token);
          setPendingToken(token);
        }
        toast.error(
          failure === "already-accepted"
            ? t("team.inviteAcceptedAlready")
            : failure === "expired"
              ? t("team.inviteAcceptExpired")
              : t("team.inviteAcceptError"),
        );
        return;
      }

      const name =
        data?.acceptWorkspaceInvite?.workspaceName ||
        data?.acceptWorkspaceInvite?.workspaceId ||
        "";
      toast.success(t("team.inviteAccepted", { workspace: name }));
      try {
        await refetch();
      } catch {
        // The mutation committed. A failed switcher refresh must not restore a
        // now-spent capability and accidentally submit it a second time.
      }
    } finally {
      setBusy(false);
    }
  }, [acceptMut, busy, refetch, t]);

  return { pendingToken, busy, accept, decline };
}
