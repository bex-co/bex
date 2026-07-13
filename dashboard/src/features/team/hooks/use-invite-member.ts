import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { InviteWorkspaceMemberDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { graphQLErrorMessage } from "@/common/lib/graphql-error";
import type { Role } from "@/features/team/types";

export interface UseInviteMemberResult {
  /** Fires inviteWorkspaceMember; resolves true on success. */
  invite: (email: string, role: Role) => Promise<boolean>;
  busy: boolean;
  /**
   * The workspace's plan refused this invite — its member cap is full, or the
   * role isn't offered on the plan. Carries the backend's exact copy so the
   * dialog can show it inline beside the upgrade CTA (w6/m15/t001) instead of
   * dead-ending in a toast. Null when the last attempt failed for some other
   * reason (toasted, as before) or hasn't failed at all.
   */
  planLimit: string | null;
}

/**
 * Wires the invite dialog to bex-api's `inviteWorkspaceMember`. On success the
 * recipient is emailed and joins on their first login (docs/ADR012-auth.md); the
 * caller refetches the pending-invite list.
 *
 * The plan refusal is singled out from every other failure because it's the one
 * the user can act on themselves: it becomes an inline alert with a route to the
 * plan-change dialog, not a toast that disappears with nowhere to go.
 */
export function useInviteMember(workspaceId: string): UseInviteMemberResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(InviteWorkspaceMemberDocument);
  const [busy, setBusy] = useState(false);
  const [planLimit, setPlanLimit] = useState<string | null>(null);

  const invite = useCallback(
    async (email: string, role: Role) => {
      setBusy(true);
      setPlanLimit(null);
      try {
        await mutate({ variables: { workspaceId, email, role } });
        toast.success(t("team.inviteSuccess", { email }));
        return true;
      } catch (err) {
        // Both plan refusals — the member cap and the role-not-on-plan guard —
        // name the plan. The service is the only place either rule lives, so its
        // message is relayed, never re-derived here; a backend test pins that the
        // refusals keep saying "plan" (members/service_test.go), since this
        // substring is the only contract between the two sides.
        const msg = graphQLErrorMessage(err) ?? "";
        if (msg.toLowerCase().includes("plan")) {
          setPlanLimit(msg);
        } else {
          toast.error(t("team.inviteError", { email }));
        }
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, workspaceId, t],
  );

  return { invite, busy, planLimit };
}
