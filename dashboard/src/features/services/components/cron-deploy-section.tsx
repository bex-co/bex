import { useState } from "react";
import { Pencil, X, Loader2, Check } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { useTranslations } from "@/common/hooks/use-translations";
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
 * Schedule + Command, editable inline. Replaces Custom Domains + Idle timeout
 * for a `cron_job`, neither of which applies to a service with no HTTP traffic
 * to serve or idle on.
 */
export function CronDeploySection({
  serviceId,
  schedule,
  command,
}: CronDeploySectionProps) {
  const { t } = useTranslations();
  const { updateCronJob, busy } = useCronJob();
  const [editing, setEditing] = useState(false);
  const [draftSchedule, setDraftSchedule] = useState("");
  const [draftCommand, setDraftCommand] = useState("");
  const [scheduleError, setScheduleError] = useState("");

  function startEdit() {
    setDraftSchedule(schedule ?? "");
    setDraftCommand(command ?? "");
    setScheduleError("");
    setEditing(true);
  }

  function cancelEdit() {
    setEditing(false);
    setScheduleError("");
  }

  async function handleSave() {
    const sched = draftSchedule.trim();
    if (!sched) {
      setScheduleError(t("services.deployScheduleRequired"));
      return;
    }
    if (!isValidCron(sched)) {
      setScheduleError(t("services.deployScheduleError"));
      return;
    }
    setScheduleError("");
    const ok = await updateCronJob(serviceId, sched, draftCommand.trim());
    if (ok) setEditing(false);
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>{t("services.deployTitle")}</CardTitle>
            <CardDescription>{t("services.deployDescription")}</CardDescription>
          </div>
          {!editing && (
            <Button
              variant="ghost"
              size="sm"
              aria-label={t("services.deployEdit")}
              onClick={startEdit}
              disabled={schedule === null}
            >
              <Pencil className="h-4 w-4" />
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-6">
        {editing ? (
          <>
            <div className="space-y-1">
              <label className="text-sm text-muted-foreground">
                {t("services.deployScheduleLabel")}
              </label>
              <Input
                value={draftSchedule}
                onChange={(e) => {
                  setDraftSchedule(e.target.value);
                  setScheduleError("");
                }}
                placeholder={t("services.deploySchedulePlaceholder")}
                className="font-mono text-sm"
                autoFocus
                onKeyDown={(e) => {
                  if (e.key === "Escape") cancelEdit();
                }}
              />
              {scheduleError ? (
                <p className="text-destructive text-sm">{scheduleError}</p>
              ) : (
                <p className="text-muted-foreground text-sm">
                  {t("services.deployScheduleHint")}
                </p>
              )}
            </div>
            <div className="space-y-1">
              <label className="text-sm text-muted-foreground">
                {t("services.deployCommandLabel")}
              </label>
              <Input
                value={draftCommand}
                onChange={(e) => setDraftCommand(e.target.value)}
                placeholder={t("services.deployCommandPlaceholder")}
                className="font-mono text-sm"
                onKeyDown={(e) => {
                  if (e.key === "Escape") cancelEdit();
                }}
              />
              <p className="text-muted-foreground text-sm">
                {t("services.deployCommandHint")}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                disabled={busy}
                onClick={() => void handleSave()}
              >
                {busy ? (
                  <Loader2 className="mr-1 h-4 w-4 animate-spin" />
                ) : (
                  <Check className="mr-1 h-4 w-4" />
                )}
                {t("services.deploySave")}
              </Button>
              <Button
                size="sm"
                variant="ghost"
                disabled={busy}
                onClick={cancelEdit}
              >
                <X className="mr-1 h-4 w-4" />
                {t("services.deployCancel")}
              </Button>
            </div>
          </>
        ) : (
          <>
            <div>
              <div className="text-sm text-muted-foreground">
                {t("services.deployScheduleLabel")}
              </div>
              <div className="mt-1 font-mono text-sm">{schedule || "—"}</div>
              <div className="mt-1 text-sm text-muted-foreground">
                {t("services.deployScheduleHint")}
              </div>
            </div>
            <div>
              <div className="text-sm text-muted-foreground">
                {t("services.deployCommandLabel")}
              </div>
              {command ? (
                <div className="bg-muted mt-1 overflow-x-auto rounded-md border px-3 py-2">
                  <code className="font-mono text-sm whitespace-pre">
                    {command}
                  </code>
                </div>
              ) : (
                <div className="mt-1 text-sm text-muted-foreground italic">
                  {t("services.deployCommandEmpty")}
                </div>
              )}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
