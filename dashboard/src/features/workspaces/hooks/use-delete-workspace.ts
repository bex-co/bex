import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DeleteWorkspaceDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  billingBalanceDue,
  mutationErrorMessage,
  type BillingBalanceDue,
} from "@/common/lib/graphql-error";
import {
  isBillingSettling,
  useSettlingRetry,
} from "@/common/hooks/use-settling-retry";

export interface UseDeleteWorkspaceResult {
  /** Fires deleteWorkspace; resolves true on success. */
  remove: (id: string, name: string, confirmation: string) => Promise<boolean>;
  busy: boolean;
  /** The backend's rejection message (e.g. a mismatched confirmation). */
  error: string | null;
  /** Set instead of `error` when the delete was refused because settling the
   *  workspace's unbilled usage failed to charge (BILLING_BALANCE_DUE). */
  balanceDue: BillingBalanceDue | null;
  /** True while the delete waits for the final usage to be charged
   *  (BILLING_SETTLING) and retries on its own. */
  settling: boolean;
}

/**
 * Wires the danger-zone delete to bex-api's `deleteWorkspace` (w6/m1,
 * admin-only): tears down the workspace's Apps, Databases, env-var secrets,
 * and FGA tuples. `confirmation` must equal Render's "sudo delete workspace
 * <name>" phrase (workspaceDeleteConfirmation) — the backend re-validates it (a
 * client-side-only check would be bypassable), so this hook never second-guesses
 * the server's answer. A BILLING_SETTLING refusal (Stripe still aggregating the
 * final usage) is retried every 15 s for up to 3 minutes (useSettlingRetry).
 */
export function useDeleteWorkspace(): UseDeleteWorkspaceResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(DeleteWorkspaceDocument);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [balanceDue, setBalanceDue] = useState<BillingBalanceDue | null>(null);
  const { run, settling } = useSettlingRetry();

  const remove = useCallback(
    async (id: string, name: string, confirmation: string) => {
      setBusy(true);
      setError(null);
      setBalanceDue(null);
      try {
        await run(() => mutate({ variables: { id, confirmation } }));
        toast.success(t("workspaces.deleteSuccess", { name }));
        return true;
      } catch (err) {
        const due = billingBalanceDue(err);
        if (due) setBalanceDue(due);
        else if (isBillingSettling(err))
          setError(t("common.billingSettlingTimeout"));
        else setError(mutationErrorMessage(err, t("workspaces.deleteError")));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, run, t],
  );

  return { remove, busy, error, balanceDue, settling };
}
