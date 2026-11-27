import { trackOwnDelete } from "@/common/lib/own-delete";
import { useState, useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DisconnectBlueprintDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  hasGraphQLErrorCode,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";

export interface UseDisconnectBlueprintResult {
  /** Disconnects the Blueprint: null once it is, else why not, for the
   *  Disconnect Blueprint modal to show (w3/m679). */
  disconnect: (id: string) => Promise<string | null>;
  busy: boolean;
}

export function useDisconnectBlueprint(): UseDisconnectBlueprintResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(DisconnectBlueprintDocument);
  const [busy, setBusy] = useState(false);

  const disconnect = useCallback(
    async (id: string): Promise<string | null> => {
      setBusy(true);
      try {
        await trackOwnDelete(() => mutate({ variables: { id } }));
        toast.success(t("blueprints.disconnectSuccess"));
        return null;
      } catch (err) {
        // Disconnect refused while an apply owns the claim: retry after it
        // settles (w8/m37 t005).
        return hasGraphQLErrorCode(err, "BLUEPRINT_SYNC_BUSY")
          ? t("blueprints.disconnectBusy")
          : mutationErrorMessage(err, t("blueprints.disconnectError"));
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { disconnect, busy };
}
