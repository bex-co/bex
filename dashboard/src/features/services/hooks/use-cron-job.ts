import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateCronJobDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

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

  const updateCronJob = useCallback(
    async (id: string, schedule: string, command: string) => {
      setBusy(true);
      try {
        await mutate({
          variables: { id, schedule, command: command || null },
        });
        toast.success(t("services.deploySuccess"), {
          description: t("services.deployConverging"),
        });
        return true;
      } catch {
        toast.error(t("services.deployError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { updateCronJob, busy };
}
