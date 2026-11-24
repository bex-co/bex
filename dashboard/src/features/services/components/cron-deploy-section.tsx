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
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { commandText } from "@/features/services/lib/command-text";
import { isNativeRuntime } from "@/features/services/lib/runtime";
import {
  cronScheduleProblem,
  describeCron,
} from "@/features/services/lib/cron";

export interface CronDeploySectionProps {
  serviceId: string;
  /** The cron expression (spec.schedule), or null while loading. */
  schedule: string | null;
  /** The entrypoint override (spec.command), or null when unset/loading. */
  command: string | null;
  /** The build runtime; a native one makes the command its start command. */
  runtime: string | null;
  /** spec.startCommand: what a native cron runs when it has no override. */
  startCommand: string | null;
}

/**
 * The cron job Settings tab's "Deploy" section (Render parity, w5/m18):
 * Schedule + Command, each an independent edit-in-place row (w5/m50). Replaces
 * Custom Domains + Idle timeout for a `cron_job`, neither of which applies to a
 * service with no HTTP traffic to serve or idle on. `updateCronJob` patches both
 * fields, so saving one row carries the other's persisted value unchanged.
 *
 * A native-runtime cron runs its start command, which the generated image's
 * Bash CMD carries; `command` is an override that replaces that CMD and runs
 * under /bin/sh, where Bash syntax fails (w8/m69). Its Command row therefore
 * shows what runs (the override when set, else the start command) and saves
 * through setStartCommand, which also drops the override. A container cron
 * (Dockerfile or prebuilt image) runs `command` and saves it via updateCronJob.
 */
export function CronDeploySection({
  serviceId,
  schedule,
  command,
  runtime,
  startCommand,
}: CronDeploySectionProps) {
  const { t } = useTranslations();
  const { updateCronJob, setStartCommand, busy, staleId } = useCronJob();
  const native = isNativeRuntime(runtime);
  const { denied: createDenied, reason: createReason } =
    useCapabilityGate("canCreate");
  const loading = schedule === null;
  // A committed save whose authoritative re-read failed: the shown values
  // may be pre-save, so neither row is writable until the retry publishes
  // (w4/276). Another service's stale save does not block this one.
  const refreshStale = staleId === serviceId;
  const refreshStaleReason = refreshStale
    ? t("services.deployRefreshPending")
    : undefined;

  const hasCommand = (command ?? "").trim() !== "";
  // What the job runs; the Schedule row keys on the override alone.
  const runCommand =
    native && !hasCommand ? (startCommand ?? "") : (command ?? "");

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
          disabled={loading || refreshStale || createDenied}
          disabledReason={refreshStaleReason ?? createReason}
          validate={(draft) => {
            const sched = draft.trim();
            if (!sched) return t("services.deployScheduleRequired");
            const problem = cronScheduleProblem(sched);
            if (problem === "never_fires")
              return t("services.cronScheduleNeverFires");
            if (problem === "format") return t("services.deployScheduleError");
            return null;
          }}
          describe={(value) => {
            const description = describeCron(value, t);
            return description
              ? t("services.deploySchedulePreview", { description })
              : null;
          }}
          // Schedule save replays the stored command when there is one, and
          // sends null when there is not — null means "keep", so a commandless
          // reschedule never becomes an explicit command write (w4/137).
          // Sending "" here would silently turn every reschedule into a clear.
          // The replay is the stored text byte for byte: trimming it would
          // rewrite a command ending in an escaped space on every reschedule
          // (w8/m68).
          onSave={(value) =>
            updateCronJob(
              serviceId,
              value,
              hasCommand ? command : null,
              "schedule",
            )
          }
        />
        <EditableFieldRow
          label={t("services.deployCommandLabel")}
          hint={t(
            native
              ? "services.deployCommandNativeHint"
              : "services.deployCommandHint",
          )}
          value={runCommand}
          placeholder={t("services.deployCommandPlaceholder")}
          editLabel={t("services.deployCommandEdit")}
          optional={!native}
          mono
          busy={busy}
          disabled={loading || refreshStale || createDenied}
          disabledReason={refreshStaleReason ?? createReason}
          // Forwards exactly what the user left behind, "" included: it clears
          // a container cron's override, and a native cron refuses it.
          normalize={commandText}
          validate={
            native
              ? (draft) =>
                  draft.trim() === ""
                    ? t("services.deployCommandRequired")
                    : null
              : undefined
          }
          // Saving a native override unchanged moves it into the start command.
          dirty={native && hasCommand ? () => true : undefined}
          onSave={(value) =>
            native
              ? setStartCommand(serviceId, value)
              : updateCronJob(
                  serviceId,
                  (schedule ?? "").trim(),
                  value,
                  "command",
                )
          }
        />
      </CardContent>
    </Card>
  );
}
