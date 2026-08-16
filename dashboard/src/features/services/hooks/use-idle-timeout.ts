import { SetIdleTimeoutDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UseIdleTimeoutResult {
  /** Fires setIdleTimeout; resolves true on success (toasted either way). */
  setIdleTimeout: (id: string, idleTTLSeconds: number) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings idle-timeout control to bex-api's `setIdleTimeout` (w1/m4.5)
 * — a bex extension that persists `spec.idleTTLSeconds`. Like the plan/scale
 * verbs the mutation is synchronous (it patches the spec and returns); the
 * operator honors the new window on the next idle check, so the toast confirms
 * the setting rather than implying an instant sleep.
 */
export function useIdleTimeout(): UseIdleTimeoutResult {
  const { run, busy } = useFieldMutation(
    SetIdleTimeoutDocument,
    (id: string, idleTTLSeconds: number) => ({ id, idleTTLSeconds }),
    {
      success: "services.idleTimeoutSuccess",
      error: "services.idleTimeoutError",
    },
  );

  return { setIdleTimeout: run, busy };
}
