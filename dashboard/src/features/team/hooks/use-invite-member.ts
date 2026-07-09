import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { InviteWorkspaceMemberDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import type { Role } from "@/features/team/types";

export interface UseInviteMemberResult {
  /** Fires inviteWorkspaceMember; resolves true on success (toasted either way). */
  invite: (email: string, role: Role) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the invite dialog to bex-api's `inviteWorkspaceMember`. On success the
 * recipient is emailed and joins on their first login (docs/auth.md); the
 * caller refetches the pending-invite list.
 */
export function useInviteMember(workspaceId: string): UseInviteMemberResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(InviteWorkspaceMemberDocument);
  const [busy, setBusy] = useState(false);

  const invite = useCallback(
    async (email: string, role: Role) => {
      setBusy(true);
      try {
        await mutate({ variables: { workspaceId, email, role } });
        toast.success(t("team.inviteSuccess", { email }));
        return true;
      } catch (e) {
        const msg = e instanceof Error ? e.message : "";
        toast.error(
          msg.toLowerCase().includes("plan")
            ? t("team.inviteErrorPlan")
            : t("team.inviteError", { email }),
        );
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, workspaceId, t],
  );

  return { invite, busy };
}
