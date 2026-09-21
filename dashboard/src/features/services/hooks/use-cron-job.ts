import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateCronJobDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useAskForProtectedConfirmation } from "@/common/providers/protected-retry-context";
import {
  ProtectedConfirmationDismissed,
  withProtectedRetry,
} from "@/features/services/lib/protected-confirmation";

export interface UseCronJobResult {
  /** Fires updateCronJob; resolves true on success (toasted either way). */
  updateCronJob: (
    id: string,
    schedule: string,
    command: string,
  ) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the cron Deploy section's Schedule + Command controls to bex-api's
 * `updateCronJob` mutation (w5/m18). The mutation patches spec.schedule and
 * spec.command; the operator applies the new schedule to the k8s CronJob on
 * its next reconcile pass. The toast confirms the write rather than implying
 * instant convergence.
 */
export function useCronJob(): UseCronJobResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(UpdateCronJobDocument);
  const [busy, setBusy] = useState(false);
  const askForConfirmation = useAskForProtectedConfirmation();

  const updateCronJob = useCallback(
    async (id: string, schedule: string, command: string) => {
      setBusy(true);
      try {
        // Changing the command changes what the job runs, which a protected
        // environment guards; rescheduling alone does not (w4/m126).
        await withProtectedRetry(askForConfirmation, (confirm) =>
          mutate({
            variables: { id, schedule, command: command || null, confirm },
          }),
        );
        toast.success(t("services.deploySuccess"), {
          description: t("services.deployConverging"),
        });
        return true;
      } catch (err) {
        if (err instanceof ProtectedConfirmationDismissed) return false;
        toast.error(mutationErrorMessage(err, t("services.deployError")));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, askForConfirmation],
  );

  return { updateCronJob, busy };
}
