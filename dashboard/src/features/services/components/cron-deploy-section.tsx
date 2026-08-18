import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { useCronJob } from "@/features/services/hooks/use-cron-job";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { isValidCron } from "@/features/services/lib/cron";

export interface CronDeploySectionProps {
  serviceId: string;
  /** The cron expression (spec.schedule), or null while loading. */
  schedule: string | null;
  /** The entrypoint override (spec.command), or null when unset/loading. */
  command: string | null;
}

/**
 * The cron job Settings tab's "Deploy" section (Render parity, w5/m18):
 * Schedule + Command, each an independent edit-in-place row (w5/m50). Replaces
 * Custom Domains + Idle timeout for a `cron_job`, neither of which applies to a
 * service with no HTTP traffic to serve or idle on. `updateCronJob` patches both
 * fields, so saving one row carries the other's persisted value unchanged.
 */
export function CronDeploySection({
  serviceId,
  schedule,
  command,
}: CronDeploySectionProps) {
  const { t } = useTranslations();
  const { updateCronJob, busy } = useCronJob();
  const { canCreate, canOperate } = useCapabilities();
  const loading = schedule === null;

  // Setting the entrypoint command is can_create (it chooses code the job runs).
  // The dashboard re-sends the existing command when saving the schedule, so
  // editing the schedule of a command-bearing cron also needs can_create; a
  // command-less schedule is a plain settings change (can_operate). Mirror that
  // so a member sees a disabled row with a reason, not a 403 on save (w9/m84).
  const hasCommand = (command ?? "").trim() !== "";
  const scheduleNeedsCreate = hasCommand;
  const scheduleBlocked = scheduleNeedsCreate ? !canCreate : !canOperate;
  const scheduleReason = scheduleBlocked
    ? t(
        scheduleNeedsCreate
          ? "capabilities.reasonCanCreate"
          : "capabilities.reasonCanOperate",
      )
    : undefined;
  const commandReason = !canCreate
    ? t("capabilities.reasonCanCreate")
    : undefined;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.deployTitle")}</CardTitle>
        <CardDescription>{t("services.deployDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <EditableFieldRow
          label={t("services.deployScheduleLabel")}
          hint={t("services.deployScheduleHint")}
          value={schedule ?? ""}
          placeholder={t("services.deploySchedulePlaceholder")}
          editLabel={t("services.deployScheduleEdit")}
          mono
          busy={busy}
          disabled={loading || scheduleBlocked}
          disabledReason={scheduleReason}
          validate={(draft) => {
            const sched = draft.trim();
            if (!sched) return t("services.deployScheduleRequired");
            if (!isValidCron(sched)) return t("services.deployScheduleError");
            return null;
          }}
          onSave={(value) =>
            updateCronJob(serviceId, value, (command ?? "").trim())
          }
        />
        <EditableFieldRow
          label={t("services.deployCommandLabel")}
          hint={t("services.deployCommandHint")}
          value={command ?? ""}
          placeholder={t("services.deployCommandPlaceholder")}
          editLabel={t("services.deployCommandEdit")}
          optional
          mono
          busy={busy}
          disabled={loading || !canCreate}
          disabledReason={commandReason}
          onSave={(value) =>
            updateCronJob(serviceId, (schedule ?? "").trim(), value)
          }
        />
      </CardContent>
    </Card>
  );
}
