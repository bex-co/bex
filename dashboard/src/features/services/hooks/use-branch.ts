import { SetBranchDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UseBranchResult {
  /** Fires setBranch; resolves true on success (toasted either way). */
  setBranch: (id: string, branch: string) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings Build & Deploy section's Branch control to bex-api's
 * `setBranch` (w5/m48/t005) — Render's editable Branch field. The mutation
 * patches `spec.branch` through the shared source verb, so the next deploy
 * builds the new branch and push-to-deploy matches pushes against it; the
 * toast confirms the write rather than implying an instant rebuild.
 */
export function useBranch(): UseBranchResult {
  const { run, busy } = useFieldMutation(
    SetBranchDocument,
    (id: string, branch: string) => ({ id, branch }),
    {
      success: "services.sourceUpdateSuccess",
      error: "services.sourceUpdateError",
    },
  );

  return { setBranch: run, busy };
}
