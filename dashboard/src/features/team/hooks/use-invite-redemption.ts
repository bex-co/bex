import { useEffect, useRef } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { AcceptWorkspaceInviteDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { INVITE_TOKEN_STORAGE_KEY } from "@/common/lib/invite-token";
import { useWorkspace } from "@/features/workspaces/context/hooks";

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
    const fromURL = new URLSearchParams(window.location.search).get("invite");
    const token =
      fromURL ?? window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY);
    if (!token) return;
    attempted.current = true;
    window.sessionStorage.removeItem(INVITE_TOKEN_STORAGE_KEY);
    void (async () => {
      try {
        const { data } = await acceptMut({ variables: { token } });
        const name =
          data?.acceptWorkspaceInvite?.workspaceName ||
          data?.acceptWorkspaceInvite?.workspaceId ||
          "";
        toast.success(t("team.inviteAccepted", { workspace: name }));
        await refetch();
      } catch (e) {
        const msg = e instanceof Error ? e.message.toLowerCase() : "";
        toast.error(
          msg.includes("already accepted")
            ? t("team.inviteAcceptedAlready")
            : msg.includes("expired")
              ? t("team.inviteAcceptExpired")
              : t("team.inviteAcceptError"),
        );
      }
    })();
  }, [acceptMut, refetch, t]);
}
