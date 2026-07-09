import { Badge } from "@/common/components/ui/badge.tsx";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card.tsx";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatRelativeAge } from "@/features/services/lib/format";
import type {
  ServiceBadgeVariant,
  ServiceView,
} from "@/features/services/types";
import type { en } from "@/i18n";

// Cron run outcome → i18n label + badge variant, keyed on the lower-cased status
// so a change in the operator's casing can't fall through silently.
const RUN_STATUS: Record<
  string,
  { label: keyof typeof en; variant: ServiceBadgeVariant }
> = {
  running: { label: "services.cronRunStatusRunning", variant: "outline" },
  succeeded: { label: "services.cronRunStatusSucceeded", variant: "default" },
  failed: { label: "services.cronRunStatusFailed", variant: "destructive" },
};

function CronRunStatusBadge({ status }: { status: string }) {
  const { t } = useTranslations();
  const s = RUN_STATUS[status.toLowerCase()] ?? RUN_STATUS.running;
  return <Badge variant={s.variant}>{t(s.label)}</Badge>;
}

/**
 * A cron_job's recent run history (status.runs, newest first). Rendered on the
 * overview tab below the details panel; shows an empty state when the cron has
 * not run yet.
 */
export function CronRunsSection({ service }: { service: ServiceView }) {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.cronRunsTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        {service.runs.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {t("services.cronRunsEmpty")}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("services.cronRunColStarted")}</TableHead>
                <TableHead>{t("services.cronRunColStatus")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {service.runs.map((run) => (
                <TableRow key={run.name}>
                  <TableCell className="tabular-nums text-muted-foreground">
                    {formatRelativeAge(run.startedAt)}
                  </TableCell>
                  <TableCell>
                    <CronRunStatusBadge status={run.status} />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
