import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateWorkspaceDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { graphQLErrorMessage } from "@/features/workspaces/lib/graphql-error";
import type { WorkspaceView } from "@/features/workspaces/types";

export interface UseCreateWorkspaceResult {
  /** Fires createWorkspace; resolves the new workspace or null on failure. */
  create: (name: string, plan: string) => Promise<WorkspaceView | null>;
  busy: boolean;
  /**
   * The backend's rejection message (e.g. the Hobby-plan-cap refusal), shown
   * inline next to the form — a toast alone would lose it once dismissed, and
   * the DoD calls for the limit error surfacing right where the user acted.
   */
  error: string | null;
}

/**
 * Wires `/new/workspace`'s submit to bex-api's `createWorkspace` (w6/m1):
 * a tenant row + the caller's admin membership, capped at five Hobby
 * workspaces per user server-side — this hook only relays whatever the
 * backend decides, never re-implements the cap client-side.
 */
export function useCreateWorkspace(): UseCreateWorkspaceResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(CreateWorkspaceDocument);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const create = useCallback(
    async (name: string, plan: string) => {
      setBusy(true);
      setError(null);
      try {
        const res = await mutate({ variables: { name, plan } });
        const w = res.data?.createWorkspace;
        if (!w?.id) throw new Error("createWorkspace returned no workspace");
        toast.success(t("workspaces.createSuccess", { name }));
        return {
          id: w.id,
          name: w.name ?? name,
          plan: w.plan ?? plan,
          role: w.role ?? "admin",
          createdAt: w.createdAt,
        };
      } catch (err) {
        setError(graphQLErrorMessage(err) ?? t("workspaces.createError"));
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { create, busy, error };
}
