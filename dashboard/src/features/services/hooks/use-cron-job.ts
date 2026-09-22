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
  /**
   * Fires updateCronJob; resolves true on success (toasted either way).
   *
   * `command` carries three distinct intents and all three reach the wire
   * unchanged, because the backend distinguishes them: `null` keeps the stored
   * command, a nonempty string replaces it, and an explicit `""` **clears** the
   * override so the job runs its image's own command (`apps/service.go`'s
   * nil-means-keep / empty-means-clear contract, and the field's own hint says
   * to leave it blank for exactly that).
   *
   * This used to send `command || null`, which collapsed the third intent into
   * the first: clearing the field saved successfully, preserved the old
   * command, and toasted success (w4/137).
   */
  updateCronJob: (
    id: string,
    schedule: string,
    command: string | null,
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
    async (id: string, schedule: string, command: string | null) => {
      setBusy(true);
      try {
        // Changing the command changes what the job runs, which a protected
        // environment guards; rescheduling alone does not (w4/m126).
        await withProtectedRetry(askForConfirmation, (confirm) =>
          mutate({
            variables: { id, schedule, command, confirm },
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
