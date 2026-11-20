import { SetRepoDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UseSetRepoResult {
  /** Fires setRepo; resolves true on success (toasted either way). */
  setRepo: (id: string, source: RepoSourceUpdate) => Promise<boolean>;
  busy: boolean;
}

export interface RepoSourceUpdate {
  repo: string;
  branch?: string;
  /**
   * Saved with the switch: an image service whose runtime builds natively
   * needs both to switch back to Git (w32/052). Omitted, each is unchanged.
   */
  buildCommand?: string;
  startCommand?: string;
}

/**
 * Wires Settings source controls to bex-api's `setRepo` (w5/m54, w5/m76).
 * Repository and branch travel as one source update; saving only changes the
 * configured source, and the next deploy consumes it.
 */
export function useSetRepo(): UseSetRepoResult {
  const { run, busy } = useFieldMutation(
    SetRepoDocument,
    (
      id: string,
      { repo, branch, buildCommand, startCommand }: RepoSourceUpdate,
    ) => ({
      id,
      repo,
      branch: branch ?? null,
      buildCommand: buildCommand ?? null,
      startCommand: startCommand ?? null,
    }),
    {
      success: "services.sourceUpdateSuccess",
      error: "services.sourceUpdateError",
    },
  );

  return { setRepo: run, busy };
}
