import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { useNavigate } from "@tanstack/react-router";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { LeaveWorkspaceDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

export interface UseLeaveWorkspaceResult {
  /** Leaves the workspace and, on success, lands the caller in one they kept.
   *  A refusal stays here (see `error`) and moves nothing. */
  leave: () => Promise<void>;
  busy: boolean;
  /** The server's refusal, rendered inline rather than as a toast — leaving is
   *  a page-level action whose failure the caller must read before retrying. */
  error: string | null;
}

/**
 * Wires the Leave action to bex-api's `leaveWorkspace` (w5/m102). The mutation
 * carries no subject: the server acts on the calling identity, so there is no
 * argument this hook could get wrong. Refusals (the workspace owner, the last
 * admin) come back coded and are surfaced inline — the UI disables the action
 * for both cases up front, so reaching one here means the membership changed
 * under the open page, which is exactly when the message matters.
 *
 * On success the caller lands in a workspace they still belong to, chosen from
 * the PRE-leave list (the left workspace is already gone from anything a
 * refetch would return): selecting from a list that has not caught up yet is
 * what blanks the switcher. With none left, the overview is still the
 * destination, since onboarding mints a personal workspace on the next
 * authenticated request.
 */
export function useLeaveWorkspace(
  workspaceId: string,
): UseLeaveWorkspaceResult {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { workspaces, setCurrentWorkspaceId, refetch } = useWorkspace();
  const [leaveMut] = useMutation(LeaveWorkspaceDocument);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const leave = useCallback(async () => {
    if (!workspaceId || busy) return;
    const fallback = workspaces.find((w) => w.id !== workspaceId);
    setBusy(true);
    setError(null);
    try {
      await leaveMut({ variables: { workspaceId } });
    } catch (e) {
      setError(mutationErrorMessage(e, t("team.leaveError")));
      return;
    } finally {
      setBusy(false);
    }
    void refetch();
    if (fallback) setCurrentWorkspaceId(fallback.id);
    void navigate({ to: "/", replace: true });
  }, [
    leaveMut,
    workspaceId,
    busy,
    workspaces,
    setCurrentWorkspaceId,
    refetch,
    navigate,
    t,
  ]);

  return { leave, busy, error };
}
