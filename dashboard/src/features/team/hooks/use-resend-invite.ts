import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { ResendWorkspaceInviteDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseResendInviteResult {
  /** Re-sends a pending invite's email (fresh expiry + freshly minted link,
   *  same id); resolves true on success. */
  resendInvite: (inviteId: string) => Promise<boolean>;
  /** The invite currently being resent, or null. */
  resending: string | null;
}

/**
 * Wires the resend action (w1/m33) to bex-api's `resendWorkspaceInvite`: the
 * recipient gets a fresh email and the invite's expiry moves forward — no
 * revoke + re-invite churn. The new mail's link supersedes the original,
 * which stops redeeming (tokens are hashed at rest, w1/041).
 */
export function useResendInvite(workspaceId: string): UseResendInviteResult {
  const { t } = useTranslations();
  const [resendMut] = useMutation(ResendWorkspaceInviteDocument);
  const [resending, setResending] = useState<string | null>(null);

  const resendInvite = useCallback(
    async (inviteId: string) => {
      setResending(inviteId);
      try {
        const { data } = await resendMut({
          variables: { workspaceId, inviteId },
        });
        const email = data?.resendWorkspaceInvite?.email ?? "";
        toast.success(t("team.resendInviteSuccess", { email }));
        return true;
      } catch {
        toast.error(t("team.resendInviteError"));
        return false;
      } finally {
        setResending(null);
      }
    },
    [resendMut, workspaceId, t],
  );

  return { resendInvite, resending };
}
