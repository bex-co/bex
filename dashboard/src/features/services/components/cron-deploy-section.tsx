import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";

export interface CronDeploySectionProps {
  /** The cron expression (spec.schedule), or null while loading. */
  schedule: string | null;
  /** The entrypoint override (spec.command), or null when unset/loading. */
  command: string | null;
}

/**
 * The cron job Settings tab's "Deploy" section (Render parity, w5/m11):
 * Schedule + Command, read-only for now (the write path is a follow-on
 * milestone). Replaces Custom Domains + Idle timeout for a `cron_job`, neither
 * of which applies to a service with no HTTP traffic to serve or idle on.
 */
export function CronDeploySection({
  schedule,
  command,
}: CronDeploySectionProps) {
  const { t } = useTranslations();

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.deployTitle")}</CardTitle>
        <CardDescription>{t("services.deployDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
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
      </CardContent>
    </Card>
  );
}
