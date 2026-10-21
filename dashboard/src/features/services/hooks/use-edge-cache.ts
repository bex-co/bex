import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  PurgeServiceCacheDocument,
  SetCacheProfileDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useAskForProtectedConfirmation } from "@/common/providers/protected-retry-context";
import {
  ProtectedConfirmationDismissed,
  withProtectedRetry,
} from "@/features/services/lib/protected-confirmation";

export interface UseEdgeCacheResult {
  setCacheProfile: (id: string, profile: string) => Promise<boolean>;
  purgeCache: (id: string) => Promise<boolean>;
  saving: boolean;
  purging: boolean;
}

/**
 * Wires the Settings Edge Caching section to bex-api's `setCacheProfile` and
 * `purgeServiceCache` (Render's web-service edge caching). Each mutation
 * returns the Service's `cacheProfile`, which Apollo merges into the cached
 * `server` document, so the section re-reads the saved profile on success.
 */
export function useEdgeCache(): UseEdgeCacheResult {
  const { t } = useTranslations();
  const [mutateProfile] = useMutation(SetCacheProfileDocument);
  const [mutatePurge] = useMutation(PurgeServiceCacheDocument);
  const [saving, setSaving] = useState(false);
  const [purging, setPurging] = useState(false);
  const askForConfirmation = useAskForProtectedConfirmation();

  const setCacheProfile = useCallback(
    async (id: string, profile: string) => {
      setSaving(true);
      try {
        await withProtectedRetry(askForConfirmation, (confirm) =>
          mutateProfile({ variables: { id, profile, confirm } }),
        );
        toast.success(t("services.edgeCachingSaved"));
        return true;
      } catch (err) {
        if (err instanceof ProtectedConfirmationDismissed) return false;
        toast.error(mutationErrorMessage(err, t("services.edgeCachingError")));
        return false;
      } finally {
        setSaving(false);
      }
    },
    [mutateProfile, t, askForConfirmation],
  );

  const purgeCache = useCallback(
    async (id: string) => {
      setPurging(true);
      try {
        await withProtectedRetry(askForConfirmation, (confirm) =>
          mutatePurge({ variables: { id, confirm } }),
        );
        toast.success(t("services.edgeCachingPurged"));
        return true;
      } catch (err) {
        if (err instanceof ProtectedConfirmationDismissed) return false;
        toast.error(
          mutationErrorMessage(err, t("services.edgeCachingPurgeError")),
        );
        return false;
      } finally {
        setPurging(false);
      }
    },
    [mutatePurge, t, askForConfirmation],
  );

  return { setCacheProfile, purgeCache, saving, purging };
}
