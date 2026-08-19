import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetServiceIpAllowListDocument } from "@/graphql/definitions";
import type { IPAllowListEntryDraft } from "@/common/lib/ip-allow-list";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Edits a web service's or static site's inbound IP allowlist (w7/m32,
 * Render's ipAllowList on webServiceDetails / staticSiteDetails). The current
 * list is read from the service detail already fetched by `useServer`; this
 * hook only handles the save path so no extra query is needed.
 */
export function useServiceNetworking() {
  const { t } = useTranslations();
  const [mutate, { loading: saving }] = useMutation(
    SetServiceIpAllowListDocument,
  );

  const saveAllowList = useCallback(
    async (id: string, entries: IPAllowListEntryDraft[]): Promise<boolean> => {
      try {
        await mutate({ variables: { id, entries } });
        toast.success(t("services.networkingSaved"));
        return true;
      } catch (e) {
        toast.error(
          t("services.networkingError", { error: (e as Error).message }),
        );
        return false;
      }
    },
    [mutate, t],
  );

  return { saving, saveAllowList };
}
