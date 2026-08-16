import { SetBuildFilterDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UseBuildFilterResult {
  /**
   * Fires setBuildFilter with the include/exclude glob lists; resolves true on
   * success (toasted either way). Passing two empty arrays clears the filter.
   */
  setBuildFilter: (
    id: string,
    paths: string[],
    ignoredPaths: string[],
  ) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings Build & Deploy section's Build Filters editor to bex-api's
 * `setBuildFilter` (w1/m34) — Render's Build Filters setting. The mutation
 * patches `spec.buildFilter` (no rebuild — the filter only changes which future
 * pushes redeploy), so the toast confirms the write rather than implying a deploy.
 */
export function useBuildFilter(): UseBuildFilterResult {
  const { run, busy } = useFieldMutation(
    SetBuildFilterDocument,
    (id: string, paths: string[], ignoredPaths: string[]) => ({
      id,
      buildFilter: { paths, ignoredPaths },
    }),
    {
      success: "services.buildFilterSuccess",
      error: "services.buildFilterError",
    },
  );

  return { setBuildFilter: run, busy };
}
