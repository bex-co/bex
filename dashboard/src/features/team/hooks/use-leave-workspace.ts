import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { LeaveWorkspaceDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

export interface UseLeaveWorkspaceResult {
  /** Leaves the workspace; resolves true on success. */
  leave: () => Promise<boolean>;
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
 */
export function useLeaveWorkspace(
  workspaceId: string,
): UseLeaveWorkspaceResult {
  const { t } = useTranslations();
  const [leaveMut] = useMutation(LeaveWorkspaceDocument);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const leave = useCallback(async () => {
    if (!workspaceId) return false;
    setBusy(true);
    setError(null);
    try {
      await leaveMut({ variables: { workspaceId } });
      return true;
    } catch (e) {
      setError(mutationErrorMessage(e, t("team.leaveError")));
      return false;
    } finally {
      setBusy(false);
    }
  }, [leaveMut, workspaceId, t]);

  return { leave, busy, error };
}
