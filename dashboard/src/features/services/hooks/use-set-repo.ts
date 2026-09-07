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
}

/**
 * Wires Settings source controls to bex-api's `setRepo` (w5/m54, w5/m76).
 * Repository and branch travel as one source update; saving only changes the
 * configured source, and the next deploy consumes it.
 */
export function useSetRepo(): UseSetRepoResult {
  const { run, busy } = useFieldMutation(
    SetRepoDocument,
    (id: string, { repo, branch }: RepoSourceUpdate) => ({
      id,
      repo,
      branch: branch ?? null,
    }),
    {
      success: "services.sourceUpdateSuccess",
      error: "services.sourceUpdateError",
    },
  );

  return { setRepo: run, busy };
}
