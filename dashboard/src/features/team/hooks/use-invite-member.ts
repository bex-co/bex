import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { InviteWorkspaceMemberDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { planLimitExtensions } from "@/common/lib/graphql-error";
import type { Role } from "@/features/team/types";

export interface UseInviteMemberResult {
  /** Fires inviteWorkspaceMember; resolves true on success. */
  invite: (email: string, role: Role) => Promise<boolean>;
  busy: boolean;
  /**
   * The workspace's plan refused this invite — its member cap is full, or the
   * role isn't offered on the plan. Carries a localized message built from the
   * error's structured params (plan name + limit), so the dialog can show it
   * inline beside the upgrade CTA instead of dead-ending in a toast. Null when
   * the last attempt failed for some other reason (toasted) or hasn't failed.
   *
   * Decoupled from backend prose: the hook keys on the PLAN_LIMIT error code
   * (extensions.code) rather than substring-matching the English message, so
   * backend copy changes have zero effect on whether the CTA shows.
   */
  planLimit: string | null;
}

/**
 * Wires the invite dialog to bex-api's `inviteWorkspaceMember`. On success the
 * recipient is emailed and joins on their first login (docs/ADR012-auth.md); the
 * caller refetches the pending-invite list.
 *
 * Plan refusals (PLAN_LIMIT code) are singled out from every other failure
 * because they're the one the user can act on: they become an inline alert with
 * a route to the plan-change dialog, not a toast that disappears with nowhere
 * to go. The localized message is built from the error's params ({plan, limit})
 * so non-English users see their language's copy, not the backend's English.
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
        const params = planLimitExtensions(err);
        if (params) {
          // Build a localized message from structured params rather than
          // relaying the backend's English prose — fixes the zh wart (non-English
          // users previously saw the raw English error string).
          const msg =
            params.limit > 0
              ? t("team.inviteErrorPlanLimitSeats", {
                  plan: params.plan,
                  limit: params.limit,
                })
              : t("team.inviteErrorPlanLimitRole", { plan: params.plan });
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
