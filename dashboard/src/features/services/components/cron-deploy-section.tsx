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
  const loading = schedule === null;

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
          disabled={loading}
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
          disabled={loading}
          onSave={(value) =>
            updateCronJob(serviceId, (schedule ?? "").trim(), value)
          }
        />
      </CardContent>
    </Card>
  );
}
