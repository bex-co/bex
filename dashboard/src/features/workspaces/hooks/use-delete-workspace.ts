import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DeleteWorkspaceDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { graphQLErrorMessage } from "@/features/workspaces/lib/graphql-error";

export interface UseDeleteWorkspaceResult {
  /** Fires deleteWorkspace; resolves true on success. */
  remove: (id: string, name: string, confirmation: string) => Promise<boolean>;
  busy: boolean;
  /** The backend's rejection message (e.g. a mismatched confirmation). */
  error: string | null;
}

/**
 * Wires the danger-zone delete to bex-api's `deleteWorkspace` (w6/m1,
 * admin-only): tears down the workspace's Apps, Databases, env-var secrets,
 * and FGA tuples. `confirmation` must equal Render's "sudo delete workspace
 * <name>" phrase (workspaceDeleteConfirmation) — the backend re-validates it (a
 * client-side-only check would be bypassable), so this hook never second-guesses
 * the server's answer.
 */
export function useDeleteWorkspace(): UseDeleteWorkspaceResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(DeleteWorkspaceDocument);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const remove = useCallback(
    async (id: string, name: string, confirmation: string) => {
      setBusy(true);
      setError(null);
      try {
        await mutate({ variables: { id, confirmation } });
        toast.success(t("workspaces.deleteSuccess", { name }));
        return true;
      } catch (err) {
        setError(graphQLErrorMessage(err) ?? t("workspaces.deleteError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { remove, busy, error };
}
