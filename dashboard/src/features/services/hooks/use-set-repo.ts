import { SetRepoDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UseSetRepoResult {
  /** Fires setRepo; resolves true on success (toasted either way). */
  setRepo: (id: string, repo: string) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings Build section's Source control to bex-api's `setRepo`
 * (w5/m54) — Render's editable Source field. Switching the repository patches
 * `spec.repo` through the shared source verb (the same one setBranch uses), so
 * the change validates and triggers the documented rebuild path.
 */
export function useSetRepo(): UseSetRepoResult {
  const { run, busy } = useFieldMutation(
    SetRepoDocument,
    (id: string, repo: string) => ({ id, repo }),
    {
      success: "services.buildDeploySuccess",
      error: "services.buildDeployError",
    },
  );

  return { setRepo: run, busy };
}
