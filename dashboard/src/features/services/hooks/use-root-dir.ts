import { SetRootDirDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UseRootDirResult {
  /** Fires setRootDir; resolves true on success (toasted either way). */
  setRootDir: (id: string, rootDir: string) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings Build & Deploy section's Root Directory control to
 * bex-api's `setRootDir` (w1/m18) — Render's Root Directory setting. The
 * mutation patches `spec.rootDir` and bumps `spec.restartedAt`, so the
 * operator rebuilds scoped to the new subdirectory; the toast confirms the
 * write rather than implying an instant rebuild.
 */
export function useRootDir(): UseRootDirResult {
  const { run, busy } = useFieldMutation(
    SetRootDirDocument,
    (id: string, rootDir: string) => ({ id, rootDir }),
    {
      success: "services.buildDeploySuccess",
      error: "services.buildDeployError",
    },
  );

  return { setRootDir: run, busy };
}
