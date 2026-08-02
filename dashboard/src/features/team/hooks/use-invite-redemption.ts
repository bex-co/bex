import { useEffect, useRef } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { AcceptWorkspaceInviteDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  retainPendingInviteToken,
  takePendingInviteToken,
} from "@/common/lib/invite-token";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { classifyInviteRedemptionError } from "./invite-redemption-error";

/**
 * Redeems a pending invite token once the caller is authenticated (w1/m33):
 * the token comes from the emailed link (`?invite=` on the current URL, or the
 * sessionStorage stash the auth pages wrote before the Kratos round-trip).
 * Success joins the workspace — even when the caller signed up under a
 * different email than the one invited — toasts the workspace joined, and
 * refreshes the switcher. A used/expired token gets a named failure toast,
 * never a silent no-op. Mounted once in the authenticated layout.
 */
export function useInviteRedemption() {
  const { t } = useTranslations();
  const { refetch } = useWorkspace();
  const [acceptMut] = useMutation(AcceptWorkspaceInviteDocument);
  const attempted = useRef(false);

  useEffect(() => {
    if (attempted.current || typeof window === "undefined") return;
    const token = takePendingInviteToken();
    if (!token) return;
    attempted.current = true;
    void (async () => {
      let data;
      try {
        ({ data } = await acceptMut({ variables: { token } }));
      } catch (e) {
        const failure = classifyInviteRedemptionError(e);
        if (failure === "ambiguous") retainPendingInviteToken(token);
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
    })();
  }, [acceptMut, refetch, t]);
}
