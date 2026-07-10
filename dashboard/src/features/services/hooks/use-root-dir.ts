import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetRootDirDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

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
  const { t } = useTranslations();
  const [mutate] = useMutation(SetRootDirDocument);
  const [busy, setBusy] = useState(false);

  const setRootDir = useCallback(
    async (id: string, rootDir: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, rootDir } });
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

  return { setRootDir, busy };
}
