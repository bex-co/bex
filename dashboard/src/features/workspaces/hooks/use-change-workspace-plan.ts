import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { ChangeWorkspacePlanDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { graphQLErrorMessage } from "@/features/workspaces/lib/graphql-error";

export interface UseChangeWorkspacePlanResult {
  /** Fires changeWorkspacePlan; resolves true on success. */
  changePlan: (id: string, plan: string) => Promise<boolean>;
  busy: boolean;
  /**
   * The backend's rejection message (a downgrade guard: member/service/
   * per-user-cap/role-set), shown inline in the change-plan dialog — the DoD
   * calls for a blocked downgrade to say exactly what to remove first.
   */
  error: string | null;
}

/**
 * Wires the workspace settings plan dialog to bex-api's `changeWorkspacePlan`
 * (w6/m12, admin-only). This hook only relays whatever the backend decides —
 * every downgrade guard (member/service/per-user-cap/role-set) is enforced
 * server-side, never re-implemented client-side.
 */
export function useChangeWorkspacePlan(): UseChangeWorkspacePlanResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(ChangeWorkspacePlanDocument);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const changePlan = useCallback(
    async (id: string, plan: string) => {
      setBusy(true);
      setError(null);
      try {
        await mutate({ variables: { id, plan } });
        toast.success(t("workspaces.changePlanSuccess", { plan }));
        return true;
      } catch (err) {
        setError(graphQLErrorMessage(err) ?? t("workspaces.changePlanError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { changePlan, busy, error };
}
