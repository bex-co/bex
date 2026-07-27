import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetRepoDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

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
  const { t } = useTranslations();
  const [mutate] = useMutation(SetRepoDocument);
  const [busy, setBusy] = useState(false);

  const setRepo = useCallback(
    async (id: string, repo: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, repo } });
        toast.success(t("services.buildDeploySuccess"));
        return true;
      } catch {
        toast.error(t("services.buildDeployError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setRepo, busy };
}
