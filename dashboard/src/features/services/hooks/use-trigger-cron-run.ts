import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";
import { RunCronJobDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  hasGraphQLErrorCode,
  isForbiddenError,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
import { LIVE_TAIL, logRangeToSearch } from "@/features/logs/lib/log-range";

export interface UseTriggerCronRunResult {
  triggering: boolean;
  trigger: () => Promise<void>;
}

/**
 * Render's Trigger Run (w3/m555): `runCronJob` fires at once, with no
 * confirm, then the cron job's Logs open on Live tail, where the run's output
 * arrives; an answer with no run opens Events instead. The server preempts an
 * active run by design (it cancels the active Job and starts the new one), as
 * Render's docs say a manual trigger does.
 *
 * A failure raises a toast that stays until dismissed: the permission
 * refusal and an unknown failure in Render's words, a suspended cron job in
 * its localized reason (w4/203), anything else in the server's own words.
 */
export function useTriggerCronRun(serviceId: string): UseTriggerCronRunResult {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const [runCronJob] = useMutation(RunCronJobDocument);
  const [triggering, setTriggering] = useState(false);

  const trigger = useCallback(async () => {
    setTriggering(true);
    try {
      const result = await runCronJob({ variables: { id: serviceId } });
      if (result.data?.runCronJob?.id) {
        await navigate({
          to: "/services/$serviceId/logs",
          params: { serviceId },
          search: logRangeToSearch(LIVE_TAIL),
        });
      } else {
        await navigate({
          to: "/services/$serviceId/events",
          params: { serviceId },
        });
      }
    } catch (error) {
      toast.error(
        isForbiddenError(error)
          ? t("services.cronTriggerForbidden")
          : hasGraphQLErrorCode(error, "CRON_SUSPENDED")
            ? t("capabilities.blockedSuspended")
            : mutationErrorMessage(error, t("services.cronTriggerError")),
        { duration: Infinity, closeButton: true },
      );
    } finally {
      setTriggering(false);
    }
  }, [navigate, runCronJob, serviceId, t]);

  return { triggering, trigger };
}
