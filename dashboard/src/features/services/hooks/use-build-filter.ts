import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetBuildFilterDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

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
  const { t } = useTranslations();
  const [mutate] = useMutation(SetBuildFilterDocument);
  const [busy, setBusy] = useState(false);

  const setBuildFilter = useCallback(
    async (id: string, paths: string[], ignoredPaths: string[]) => {
      setBusy(true);
      try {
        await mutate({
          variables: { id, buildFilter: { paths, ignoredPaths } },
        });
        toast.success(t("services.buildFilterSuccess"));
        return true;
      } catch {
        toast.error(t("services.buildFilterError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setBuildFilter, busy };
}
