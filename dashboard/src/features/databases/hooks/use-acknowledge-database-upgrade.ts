import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useTranslations } from "@/common/hooks/use-translations";
import { AcknowledgeDatabaseUpgradeDocument } from "@/graphql/definitions";

/**
 * Dismisses a finished upgrade's result (Render's acknowledgeDatabaseUpgrade,
 * w3/m676). The answer carries the run, so Apollo updates the database the
 * page reads and the banner goes.
 */
export function useAcknowledgeDatabaseUpgrade() {
  const { t } = useTranslations();
  const [mutate, { loading }] = useMutation(AcknowledgeDatabaseUpgradeDocument);
  const acknowledge = useCallback(
    async (id: string, runId: string) => {
      try {
        await mutate({ variables: { id, runId } });
      } catch (err) {
        toast.error(
          mutationErrorMessage(err, t("databases.versionUpgradeDismissError")),
        );
      }
    },
    [mutate, t],
  );
  return { acknowledge, busy: loading };
}
