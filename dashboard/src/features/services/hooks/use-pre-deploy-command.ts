import { SetPreDeployCommandDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UsePreDeployCommandResult {
  /** Fires setPreDeployCommand; resolves true on success (toasted either way). */
  setPreDeployCommand: (id: string, command: string) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings Build & Deploy section's Pre-Deploy Command control to
 * bex-api's `setPreDeployCommand` (w1/m33) — Render's Pre-Deploy Command. The
 * mutation patches `spec.preDeployCommand`; the command runs against the new
 * image on the resulting rollout, and a non-zero exit fails the deploy while the
 * previous revision keeps serving. The toast confirms the write.
 */
export function usePreDeployCommand(): UsePreDeployCommandResult {
  const { run, busy } = useFieldMutation(
    SetPreDeployCommandDocument,
    (id: string, command: string) => ({ id, command }),
    { success: "services.preDeploySuccess", error: "services.preDeployError" },
  );

  return { setPreDeployCommand: run, busy };
}
